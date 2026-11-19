package reverseproxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gdy666/home-network-proxy/internal/config"
)

// Service 反向代理服务配置
type Service struct {
	Name        string            `json:"name"`
	Domain      string            `json:"domain"`
	TargetURL   string            `json:"target_url"`
	Enabled     bool              `json:"enabled"`
	SSLEnabled  bool              `json:"ssl_enabled"`
	CertFile    string            `json:"cert_file"`
	KeyFile     string            `json:"key_file"`
	Headers     map[string]string `json:"headers"`
	WebSocket   bool              `json:"websocket"`
	Timeout     int               `json:"timeout"` // 秒
	Retry       int               `json:"retry"`
	PathPrefix  string            `json:"path_prefix"` // 路径前缀匹配
}

// Manager 反向代理管理器
type Manager struct {
	cfg      *config.Config
	services map[string]*Service
	proxies  map[string]*httputil.ReverseProxy
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  bool
	server   *http.Server
}

// NewManager 创建反向代理管理器
func NewManager(cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		cfg:      cfg,
		services: make(map[string]*Service),
		proxies:  make(map[string]*httputil.ReverseProxy),
		ctx:      ctx,
		cancel:   cancel,
	}
	m.parseServices()
	return m
}

// parseServices 解析服务配置
func (m *Manager) parseServices() {
	for _, svcStr := range m.cfg.ReverseProxy.Services {
		// 格式: name|domain|target_url|enabled|ssl_enabled|cert|key|ws|timeout|retry|path_prefix
		parts := strings.Split(svcStr, "|")
		if len(parts) < 3 {
			continue
		}

		svc := &Service{
			Name:       parts[0],
			Domain:     parts[1],
			TargetURL:  parts[2],
			Enabled:    true,
			SSLEnabled: false,
			WebSocket:  false,
			Timeout:    30,
			Retry:      3,
		}

		if len(parts) > 3 && parts[3] == "false" {
			svc.Enabled = false
		}
		if len(parts) > 4 && parts[4] == "true" {
			svc.SSLEnabled = true
		}
		if len(parts) > 5 {
			svc.CertFile = parts[5]
		}
		if len(parts) > 6 {
			svc.KeyFile = parts[6]
		}
		if len(parts) > 7 && parts[7] == "true" {
			svc.WebSocket = true
		}
		if len(parts) > 8 {
			fmt.Sscanf(parts[8], "%d", &svc.Timeout)
		}
		if len(parts) > 9 {
			fmt.Sscanf(parts[9], "%d", &svc.Retry)
		}
		if len(parts) > 10 {
			svc.PathPrefix = parts[10]
		}

		svc.Headers = make(map[string]string)
		m.services[svc.Name] = svc
	}
}

// Start 启动反向代理
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	if !m.cfg.ReverseProxy.Enabled || len(m.services) == 0 {
		log.Println("反向代理已禁用或无服务配置")
		return nil
	}

	// 创建反向代理
	for name, svc := range m.services {
		if !svc.Enabled {
			continue
		}
		proxy, err := m.createProxy(svc)
		if err != nil {
			log.Printf("创建反向代理 %s 失败: %v", name, err)
			continue
		}
		m.proxies[name] = proxy
		log.Printf("反向代理 %s 已创建: %s -> %s", name, svc.Domain, svc.TargetURL)
	}

	if len(m.proxies) == 0 {
		log.Println("没有可用的反向代理服务")
		return nil
	}

	// 启动 HTTP 服务器
	m.server = &http.Server{
		Addr:    ":80", // 反向代理通常监听 80/443
		Handler: m,
	}

	m.running = true
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		log.Println("反向代理服务器启动在 :80")
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("反向代理服务器错误: %v", err)
		}
	}()

	// 如果有 SSL 服务，启动 HTTPS 服务器
	m.startHTTPS()

	log.Println("反向代理服务已启动")
	return nil
}

// startHTTPS 启动 HTTPS 服务器
func (m *Manager) startHTTPS() {
	hasSSL := false
	for _, svc := range m.services {
		if svc.Enabled && svc.SSLEnabled && svc.CertFile != "" && svc.KeyFile != "" {
			hasSSL = true
			break
		}
	}

	if !hasSSL {
		return
	}

	// 简化：实际需要为每个域名配置证书
	// 这里演示启动一个 HTTPS 服务器
	go func() {
		httpsServer := &http.Server{
			Addr:    ":443",
			Handler: m,
		}
		// 使用第一个找到的证书
		var certFile, keyFile string
		for _, svc := range m.services {
			if svc.SSLEnabled && svc.CertFile != "" && svc.KeyFile != "" {
				certFile = svc.CertFile
				keyFile = svc.KeyFile
				break
			}
		}
		if certFile != "" && keyFile != "" {
			log.Println("HTTPS 反向代理服务器启动在 :443")
			if err := httpsServer.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTPS 服务器错误: %v", err)
			}
		}
	}()
}

// createProxy 创建单个反向代理
func (m *Manager) createProxy(svc *Service) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(svc.TargetURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ModifyResponse = func(resp *http.Response) error {
		// 添加自定义头
		for k, v := range svc.Headers {
			resp.Header.Set(k, v)
		}
		return nil
	}

	// 自定义 Director
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		// 添加转发头
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "http")
		if svc.SSLEnabled {
			req.Header.Set("X-Forwarded-Proto", "https")
		}
		req.Header.Set("X-Real-IP", getClientIP(req))
	}

	// 错误处理
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("反向代理错误 [%s]: %v", svc.Name, err)
		// 重试逻辑
		for i := 0; i < svc.Retry; i++ {
			time.Sleep(time.Duration(i+1) * time.Second)
			originalDirector(r)
			// 这里简化处理，实际需要重新执行代理
		}
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	// 设置超时
	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: time.Duration(svc.Timeout) * time.Second,
	}

	return proxy, nil
}

// ServeHTTP 实现 http.Handler 接口
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	// 去除端口
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	path := r.URL.Path

	// 查找匹配的服务
	m.mu.RLock()
	var matched *Service
	var matchedProxy *httputil.ReverseProxy
	for name, svc := range m.services {
		if !svc.Enabled {
			continue
		}
		// 域名匹配
		if svc.Domain != "" && svc.Domain != "*" && svc.Domain != host {
			continue
		}
		// 路径前缀匹配
		if svc.PathPrefix != "" && !strings.HasPrefix(path, svc.PathPrefix) {
			continue
		}
		matched = svc
		matchedProxy = m.proxies[name]
		break
	}
	m.mu.RUnlock()

	if matched == nil || matchedProxy == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// WebSocket 支持
	if matched.WebSocket && isWebSocketUpgrade(r) {
		// 处理 WebSocket 升级
		matchedProxy.ServeHTTP(w, r)
		return
	}

	matchedProxy.ServeHTTP(w, r)
}

// isWebSocketUpgrade 检查是否为 WebSocket 升级请求
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Connection"), "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// getClientIP 获取客户端真实 IP
func getClientIP(r *http.Request) string {
	// 优先检查 X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	// 检查 X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// 从 RemoteAddr 获取
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// Stop 停止反向代理
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.cancel()
	if m.server != nil {
		m.server.Close()
	}
	m.wg.Wait()
	m.running = false
	log.Println("反向代理服务已停止")
}

// Reload 重新加载配置
func (m *Manager) Reload(cfg *config.Config) error {
	m.Stop()
	m.cfg = cfg
	m.services = make(map[string]*Service)
	m.proxies = make(map[string]*httputil.ReverseProxy)
	m.parseServices()
	return m.Start(m.ctx)
}

// GetStatus 获取状态
func (m *Manager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	services := []map[string]interface{}{}
	for name, svc := range m.services {
		services = append(services, map[string]interface{}{
			"name":       name,
			"domain":     svc.Domain,
			"target_url": svc.TargetURL,
			"enabled":    svc.Enabled,
			"ssl":        svc.SSLEnabled,
		})
	}

	return map[string]interface{}{
		"enabled":  m.cfg.ReverseProxy.Enabled,
		"running":  m.running,
		"services": services,
	}
}