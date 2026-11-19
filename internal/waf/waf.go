package waf

import (
	"context"
	"log"
	"net"
	"sync"
)

// Manager WAF 管理器。
// 提供 IP 白名单/黑名单过滤。
type Manager struct {
	whitelist map[string]bool
	blacklist map[string]bool
	enabled   bool
	mode      string // detection | prevention
	mu        sync.RWMutex
}

// NewManager 创建 WAF 管理器。
func NewManager() *Manager {
	return &Manager{
		whitelist: make(map[string]bool),
		blacklist: make(map[string]bool),
		enabled:   false,
		mode:      "detection",
	}
}

// Start 启动。
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
	log.Println("WAF 已启动，模式:", m.mode)
	return nil
}

// Stop 停止。
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
	return nil
}

// Reload 重新加载。
func (m *Manager) Reload() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return nil
}

// AddWhitelist 添加白名单 IP。
func (m *Manager) AddWhitelist(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.whitelist[ip] = true
}

// AddBlacklist 添加黑名单 IP。
func (m *Manager) AddBlacklist(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blacklist[ip] = true
}

// CheckIP 检查 IP 是否允许访问。
func (m *Manager) CheckIP(ip string) (allowed bool, reason string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return true, "WAF 已禁用"
	}

	if m.whitelist[ip] {
		return true, "白名单"
	}
	if m.blacklist[ip] {
		if m.mode == "prevention" {
			return false, "黑名单"
		}
		return true, "黑名单（仅记录）"
	}
	return true, "默认允许"
}

// Status 返回状态。
func (m *Manager) Status() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]interface{}{
		"enabled":   m.enabled,
		"mode":      m.mode,
		"whitelist": len(m.whitelist),
		"blacklist": len(m.blacklist),
	}
}

// 防止未使用
var _ = net.IPv4len
