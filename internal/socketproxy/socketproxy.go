package socketproxy

import (
	"context"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gdy666/home-network-proxy/internal/config"
)

// Service Socket 代理服务配置
type Service struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"` // tcp, udp
	ListenAddr string `json:"listen_addr"`
	TargetAddr string `json:"target_addr"`
	Enabled    bool   `json:"enabled"`
}

// Manager Socket 代理管理器
type Manager struct {
	cfg      *config.Config
	services map[string]*Service
	listeners map[string]io.Closer
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  bool
}

// NewManager 创建 Socket 代理管理器
func NewManager(cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		cfg:       cfg,
		services:  make(map[string]*Service),
		listeners: make(map[string]io.Closer),
		ctx:       ctx,
		cancel:    cancel,
	}
	m.parseServices()
	return m
}

// parseServices 解析服务配置
func (m *Manager) parseServices() {
	for _, svcStr := range m.cfg.SocketProxy.Services {
		// 格式: name|protocol|listen_addr|target_addr|enabled
		parts := strings.Split(svcStr, "|")
		if len(parts) < 4 {
			continue
		}

		svc := &Service{
			Name:       parts[0],
			Protocol:   parts[1],
			ListenAddr: parts[2],
			TargetAddr: parts[3],
			Enabled:    true,
		}

		if len(parts) > 4 && parts[4] == "false" {
			svc.Enabled = false
		}

		m.services[svc.Name] = svc
	}
}

// Start 启动 Socket 代理
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	if !m.cfg.SocketProxy.Enabled || len(m.services) == 0 {
		log.Println("Socket 代理已禁用或无服务配置")
		return nil
	}

	for name, svc := range m.services {
		if !svc.Enabled {
			continue
		}

		var err error
		switch strings.ToLower(svc.Protocol) {
		case "tcp":
			err = m.startTCPProxy(name, svc)
		case "udp":
			err = m.startUDPProxy(name, svc)
		default:
			log.Printf("不支持的协议: %s", svc.Protocol)
			continue
		}

		if err != nil {
			log.Printf("启动 Socket 代理 %s 失败: %v", name, err)
			continue
		}
		log.Printf("Socket 代理 %s 已启动: %s %s -> %s", name, svc.Protocol, svc.ListenAddr, svc.TargetAddr)
	}

	if len(m.listeners) == 0 {
		log.Println("没有可用的 Socket 代理服务")
		return nil
	}

	m.running = true
	log.Println("Socket 代理服务已启动")
	return nil
}

// startTCPProxy 启动 TCP 代理
func (m *Manager) startTCPProxy(name string, svc *Service) error {
	listener, err := net.Listen("tcp", svc.ListenAddr)
	if err != nil {
		return err
	}

	m.listeners[name] = listener

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-m.ctx.Done():
					return
				default:
					log.Printf("TCP 代理 %s 接受连接失败: %v", name, err)
					continue
				}
			}

			m.wg.Add(1)
			go m.handleTCPConnection(name, conn, svc.TargetAddr)
		}
	}()

	return nil
}

// handleTCPConnection 处理 TCP 连接
func (m *Manager) handleTCPConnection(name string, clientConn net.Conn, targetAddr string) {
	defer m.wg.Done()
	defer clientConn.Close()

	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("TCP 代理 %s 连接目标失败: %v", name, err)
		return
	}
	defer targetConn.Close()

	// 双向转发
	done := make(chan struct{}, 2)
	go func() {
		copyBuffer(clientConn, targetConn)
		done <- struct{}{}
	}()
	go func() {
		copyBuffer(targetConn, clientConn)
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-m.ctx.Done():
	}
}

// startUDPProxy 启动 UDP 代理
func (m *Manager) startUDPProxy(name string, svc *Service) error {
	addr, err := net.ResolveUDPAddr("udp", svc.ListenAddr)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	m.listeners[name] = conn

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		buf := make([]byte, 65535)
		for {
			select {
			case <-m.ctx.Done():
				return
			default:
				conn.SetReadDeadline(time.Now().Add(1 * time.Second))
				n, clientAddr, err := conn.ReadFromUDP(buf)
				if err != nil {
					if ne, ok := err.(net.Error); ok && ne.Timeout() {
						continue
					}
					log.Printf("UDP 代理 %s 读取失败: %v", name, err)
					continue
				}

				// 转发到目标
				targetAddr, err := net.ResolveUDPAddr("udp", svc.TargetAddr)
				if err != nil {
					log.Printf("UDP 代理 %s 解析目标地址失败: %v", name, err)
					continue
				}

				targetConn, err := net.DialUDP("udp", nil, targetAddr)
				if err != nil {
					log.Printf("UDP 代理 %s 连接目标失败: %v", name, err)
					continue
				}

				_, err = targetConn.Write(buf[:n])
				if err != nil {
					log.Printf("UDP 代理 %s 写入目标失败: %v", name, err)
					targetConn.Close()
					continue
				}

				// 读取响应并转发回客户端
				respBuf := make([]byte, 65535)
				targetConn.SetReadDeadline(time.Now().Add(5 * time.Second))
				m, err := targetConn.Read(respBuf)
				targetConn.Close()
				if err != nil {
					log.Printf("UDP 代理 %s 读取响应失败: %v", name, err)
					continue
				}

				conn.WriteToUDP(respBuf[:m], clientAddr)
			}
		}
	}()

	return nil
}

// copyBuffer 复制数据
func copyBuffer(dst, src net.Conn) {
	buf := make([]byte, 32768)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// Stop 停止 Socket 代理
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.cancel()

	// 关闭所有监听器
	for name, listener := range m.listeners {
		listener.Close()
		log.Printf("Socket 代理 %s 监听器已关闭", name)
	}

	m.wg.Wait()
	m.running = false
	log.Println("Socket 代理服务已停止")
}

// Reload 重新加载配置
func (m *Manager) Reload(cfg *config.Config) error {
	m.Stop()
	m.cfg = cfg
	m.services = make(map[string]*Service)
	m.listeners = make(map[string]io.Closer)
	m.parseServices()
	return m.Start(m.ctx)
}

// GetStatus 获取状态
func (m *Manager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	services := []map[string]interface{}{}
	for name, svc := range m.services {
		_, running := m.listeners[name]
		services = append(services, map[string]interface{}{
			"name":        name,
			"protocol":    svc.Protocol,
			"listen_addr": svc.ListenAddr,
			"target_addr": svc.TargetAddr,
			"enabled":     svc.Enabled,
			"running":     running,
		})
	}

	return map[string]interface{}{
		"enabled":  m.cfg.SocketProxy.Enabled,
		"running":  m.running,
		"services": services,
	}
}