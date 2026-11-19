package ddns

import (
	"context"
	"testing"

	"github.com/gdy666/home-network-proxy/internal/config"
)

func TestNewManager(t *testing.T) {
	cfg := defaultTestConfig()
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if len(m.providers) == 0 {
		t.Error("expected providers to be registered")
	}
}

func TestStartStop(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.DDNS.Enabled = false
	m := NewManager(cfg)
	
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	
	m.Stop()
}

func TestGetCurrentIPs(t *testing.T) {
	cfg := defaultTestConfig()
	m := NewManager(cfg)
	
	ipv4, ipv6 := m.getCurrentIPs()
	if ipv4 != "" || ipv6 != "" {
		t.Logf("获取到公网 IP: v4=%s, v6=%s", ipv4, ipv6)
	}
}

func TestFetchIP(t *testing.T) {
	ip, err := fetchIP("https://api.ipify.org")
	if err != nil {
		t.Skipf("网络不可用: %v", err)
	}
	if ip == "" {
		t.Error("expected non-empty IP")
	}
}

func defaultTestConfig() *config.Config {
	return &config.Config{
		DDNS: config.DDNSConfig{
			Enabled:     true,
			Providers:   map[string]string{},
			Interval:    300,
			IPv4Enabled: true,
			IPv6Enabled: true,
		},
		Storage: config.StorageConfig{
			Path: "/tmp/test-hnproxy",
		},
	}
}
