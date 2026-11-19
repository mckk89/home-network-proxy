package socketproxy

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/gdy666/home-network-proxy/internal/config"
)

func TestNewManager(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestParseServices(t *testing.T) {
	cfg := &config.Config{
		SocketProxy: config.SocketProxyConfig{
			Enabled: true,
			Services: []string{
				"test|tcp|0.0.0.0:12345|127.0.0.1:80|true",
				"test2|udp|0.0.0.0:12346|127.0.0.1:53|false",
			},
		},
	}
	m := NewManager(cfg)
	
	if len(m.services) != 2 {
		t.Errorf("expected 2 services, got %d", len(m.services))
	}
	
	svc := m.services["test"]
	if svc == nil {
		t.Fatal("service 'test' not found")
	}
	if svc.Protocol != "tcp" {
		t.Errorf("expected protocol tcp, got %s", svc.Protocol)
	}
	if svc.Enabled != true {
		t.Error("expected enabled=true")
	}
}

func TestStartStop(t *testing.T) {
	cfg := &config.Config{
		SocketProxy: config.SocketProxyConfig{
			Enabled: true,
			Services: []string{
				"test|tcp|127.0.0.1:18098|127.0.0.1:18099|true",
			},
		},
	}
	m := NewManager(cfg)
	
	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	
	// 验证监听器已创建
	if len(m.listeners) != 1 {
		t.Errorf("expected 1 listener, got %d", len(m.listeners))
	}
	
	// 测试 TCP 连接
	conn, err := net.Dial("tcp", "127.0.0.1:18098")
	if err == nil {
		conn.Close()
	}
	
	m.Stop()
}

func TestCopyBuffer(t *testing.T) {
	// 创建 TCP 管道对
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	server, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	
	// 测试数据复制
	done := make(chan struct{})
	go func() {
		copyBuffer(client, server)
		close(done)
	}()
	
	client.Write([]byte("hello"))
	client.Close()
	
	select {
	case <-done:
		// 成功
	case <-time.After(time.Second):
		t.Error("copyBuffer did not complete")
	}
}
