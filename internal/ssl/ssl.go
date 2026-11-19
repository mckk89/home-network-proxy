package ssl

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gdy666/home-network-proxy/internal/config"
)

// Manager SSL 证书管理器。
// 支持自签名证书生成（简化实现）。
// 生产环境应集成 ACME 协议（Let's Encrypt 等）。
type Manager struct {
	cfg     *config.Config
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

// NewManager 创建 SSL 管理器。
func NewManager(cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动 SSL 管理器。
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}
	if !m.cfg.Web.EnableHTTPS {
		log.Println("SSL 已禁用，跳过")
		return nil
	}
	m.running = true
	return nil
}

// Stop 停止。
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil
	}
	m.cancel()
	m.wg.Wait()
	m.running = false
	return nil
}

// Reload 重新加载。
func (m *Manager) Reload(cfg *config.Config) error {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	return nil
}

// GenerateSelfSignedCert 生成自签名证书。
func (m *Manager) GenerateSelfSignedCert(domain string, days int) error {
	// 生成 ECDSA P-256 密钥
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	// 生成证书
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: domain,
			Organization: []string{"家庭网络反代"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(0, 0, days),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames: []string{domain},
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
		},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}

	// 保存证书
	certPath := filepath.Join(m.cfg.Storage.Path, "certs", domain+".crt")
	keyPath := filepath.Join(m.cfg.Storage.Path, "certs", domain+".key")
	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return err
	}

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return err
	}

	log.Printf("自签名证书已生成: %s -> %s (%d 天有效期)", domain, certPath, days)
	return nil
}

// Status 返回状态。
func (m *Manager) Status() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]interface{}{
		"running": m.running,
		"enabled": m.cfg.Web.EnableHTTPS,
		"cert":    m.cfg.Web.CertFile,
		"key":     m.cfg.Web.KeyFile,
	}
}

// 防止未使用
var _ = fmt.Sprintf
