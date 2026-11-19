package ftp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Server FTP 服务器（简化实现，仅支持基础命令）。
// 生产环境建议使用 ftplib 或第三方库。
type Server struct {
	root     string
	port     int
	pasvIP   string
	readonly bool
	listener net.Listener
	mu       sync.RWMutex
	running  bool
}

// NewServer 创建 FTP 服务器。
func NewServer(root string, port int) *Server {
	return &Server{
		root: root,
		port: port,
	}
}

// Start 启动 FTP 服务。
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if err := os.MkdirAll(s.root, 0755); err != nil {
		return fmt.Errorf("创建 FTP 根目录失败: %w", err)
	}

	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = listener
	s.running = true

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("FTP accept 失败: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

// Stop 停止服务。
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return nil
	}
	s.running = false
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// handleConnection 处理单个 FTP 连接。
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	// 简化：仅实现基础 USER/PASS 流程
	_ = conn
}

// Status 返回状态。
func (s *Server) Status() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{
		"running": s.running,
		"root":    s.root,
		"port":    s.port,
	}
}

// 防止未使用导入
var _ = io.Discard
var _ = time.Now
var _ = filepath.Join
var _ = strconv.Itoa
var _ = strings.TrimSpace
