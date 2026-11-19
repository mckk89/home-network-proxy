package ddns

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gdy666/home-network-proxy/internal/config"
)

// Provider DDNS 服务商接口
type Provider interface {
	// Name 返回服务商标识
	Name() string
	// UpdateIP 更新指定域名的解析记录
	UpdateIP(domain, ip string) error
	// GetCurrentIP 获取域名当前解析的 IP
	GetCurrentIP(domain string) (string, error)
}

// Manager DDNS 管理器
type Manager struct {
	cfg       *config.Config
	providers map[string]Provider
	creds     *Credentials
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	// 记录上次更新的 IP，避免重复推送
	lastIPs map[string]string
}

// NewManager 创建 DDNS 管理器
func NewManager(cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		cfg:       cfg,
		providers: make(map[string]Provider),
		creds:     &Credentials{},
		ctx:       ctx,
		cancel:    cancel,
		lastIPs:   make(map[string]string),
	}
	m.registerProviders()
	m.loadCredentials()
	return m
}

// registerProviders 注册内置 DDNS 服务商
func (m *Manager) registerProviders() {
	alidns := NewAliDNSProvider()
	dnspod := NewDNSPodProvider()
	cloudflare := NewCloudflareProvider()
	callback := NewCallbackProvider()
	he := NewHurricaneElectricProvider()
	noip := NewNoIPProvider()
	duckdns := NewDuckDNSProvider()

	m.providers["alidns"] = alidns
	m.providers["dnspod"] = dnspod
	m.providers["cloudflare"] = cloudflare
	m.providers["callback"] = callback
	m.providers["he"] = he
	m.providers["noip"] = noip
	m.providers["duckdns"] = duckdns

	// 后续 loadCredentials 会注入实际凭据
}

// loadCredentials 加载认证凭据
func (m *Manager) loadCredentials() {
	creds := LoadCredentials(m.cfg.Storage.Path)
	m.creds = creds

	// 注入到各 Provider
	for name, p := range m.providers {
		if injectable, ok := p.(interface {
			WithCredentials(*Credentials)
		}); ok {
			injectable.WithCredentials(creds)
			log.Printf("DDNS 服务商 %s 已加载凭据", name)
		}
	}
}

// Start 启动 DDNS 服务
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	if !m.cfg.DDNS.Enabled {
		log.Println("DDNS 已禁用")
		return nil
	}

	if len(m.cfg.DDNS.Providers) == 0 {
		log.Println("DDNS 已启用但无域名配置，跳过")
		return nil
	}

	m.running = true
	m.wg.Add(1)
	go m.runLoop()

	log.Println("DDNS 服务已启动，监控", len(m.cfg.DDNS.Providers), "个域名")
	return nil
}

// Stop 停止 DDNS 服务
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.cancel()
	m.wg.Wait()
	m.running = false
	log.Println("DDNS 服务已停止")
}

// Reload 重新加载配置
func (m *Manager) Reload(cfg *config.Config) error {
	m.mu.Lock()
	m.cfg = cfg
	m.lastIPs = make(map[string]string)
	m.mu.Unlock()

	// 重新加载凭据
	m.loadCredentials()

	// 如果正在运行，重启循环
	if m.running {
		m.Stop()
		if err := m.Start(m.ctx); err != nil {
			return err
		}
	}
	return nil
}

// runLoop 运行主循环
func (m *Manager) runLoop() {
	defer m.wg.Done()

	// 立即执行一次
	m.updateAll()

	interval := time.Duration(m.cfg.DDNS.Interval) * time.Second
	if interval < time.Minute {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateAll()
		}
	}
}

// updateAll 更新所有配置的域名
func (m *Manager) updateAll() {
	ipv4, ipv6 := m.getCurrentIPs()
	if ipv4 == "" && ipv6 == "" {
		log.Println("DDNS: 无法获取公网 IP，跳过本轮更新")
		return
	}

	m.mu.RLock()
	domains := m.cfg.DDNS.Providers
	ipv4Enabled := m.cfg.DDNS.IPv4Enabled
	ipv6Enabled := m.cfg.DDNS.IPv6Enabled
	m.mu.RUnlock()

	for domain, providerName := range domains {
		provider, ok := m.providers[providerName]
		if !ok {
			log.Printf("DDNS: 未知服务商 %s (域名 %s)，跳过", providerName, domain)
			continue
		}

		// 决定目标 IP
		isIPv6Domain := strings.Contains(domain, ":") || strings.Contains(domain, "ipv6") || strings.Contains(domain, "v6")
		var targetIP string
		if isIPv6Domain {
			if ipv6Enabled && ipv6 != "" {
				targetIP = ipv6
			}
		} else {
			if ipv4Enabled && ipv4 != "" {
				targetIP = ipv4
			}
			// 回退：如果没有 IPv4，尝试 IPv6
			if targetIP == "" && ipv6Enabled && ipv6 != "" {
				targetIP = ipv6
			}
		}

		if targetIP == "" {
			log.Printf("DDNS: 域名 %s 无可用目标 IP", domain)
			continue
		}

		// 检查是否有变化
		m.mu.Lock()
		prevIP := m.lastIPs[domain]
		m.mu.Unlock()

		if prevIP == targetIP && prevIP != "" {
			// IP 未变化，跳过
			continue
		}

		// 执行更新
		if err := provider.UpdateIP(domain, targetIP); err != nil {
			log.Printf("DDNS: 更新域名 %s -> %s 失败 (%s): %v", domain, targetIP, providerName, err)
			continue
		}

		m.mu.Lock()
		m.lastIPs[domain] = targetIP
		m.mu.Unlock()

		log.Printf("DDNS: 域名 %s 更新成功: %s -> %s [%s]", domain, prevIP, targetIP, providerName)
	}
}

// getCurrentIPs 获取当前公网 IP
func (m *Manager) getCurrentIPs() (ipv4, ipv6 string) {
	// 尝试多个 IP 检测服务
	ipv4Services := []string{
		"https://api.ipify.org",
		"https://ipv4.icanhazip.com",
		"https://ifconfig.me/ip",
		"https://checkip.amazonaws.com",
	}

	ipv6Services := []string{
		"https://api6.ipify.org",
		"https://ipv6.icanhazip.com",
	}

	for _, svc := range ipv4Services {
		if ip, err := fetchIP(svc); err == nil {
			if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() != nil {
				ipv4 = ip
				break
			}
		}
	}

	for _, svc := range ipv6Services {
		if ip, err := fetchIP(svc); err == nil {
			if parsed := net.ParseIP(ip); parsed != nil && parsed.To16() != nil && parsed.To4() == nil {
				ipv6 = ip
				break
			}
		}
	}

	return
}

// fetchIP 从服务获取 IP
func fetchIP(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("响应不是有效 IP: %s", ip)
	}

	return ip, nil
}

// isIPv4 判断是否为 IPv4
func isIPv4(ip string) bool {
	return net.ParseIP(ip).To4() != nil
}

// isIPv6 判断是否为 IPv6
func isIPv6(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To16() != nil && parsed.To4() == nil
}

// isIPv6Domain 判断域名是否为 IPv6 专用
func isIPv6Domain(domain string) bool {
	return strings.Contains(domain, "ipv6") || strings.Contains(domain, "v6")
}

// GetStatus 获取状态
func (m *Manager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 计算域名数量
	domainCount := 0
	for k := range m.cfg.DDNS.Providers {
		if _, ok := m.lastIPs[k]; ok {
			domainCount++
		}
	}

	return map[string]interface{}{
		"enabled":   m.cfg.DDNS.Enabled,
		"running":   m.running,
		"interval":  m.cfg.DDNS.Interval,
		"providers": len(m.providers),
		"domains":   len(m.cfg.DDNS.Providers),
		"updated":   domainCount,
		"ipv4":      m.cfg.DDNS.IPv4Enabled,
		"ipv6":      m.cfg.DDNS.IPv6Enabled,
	}
}
