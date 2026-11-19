package ddns

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Credentials DDNS 服务商认证凭据，保存于配置目录 secrets.yaml。
type Credentials struct {
	AliDNS struct {
		AccessKeyID     string `yaml:"access_key_id"`
		AccessKeySecret string `yaml:"access_key_secret"`
	} `yaml:"alidns"`

	DNSPod struct {
		APIKey  string `yaml:"api_key"` // 格式: ID,Token
	} `yaml:"dnspod"`

	Cloudflare struct {
		APIToken string `yaml:"api_token"`
		ZoneID   string `yaml:"zone_id"`
	} `yaml:"cloudflare"`

	Callback struct {
		URL string `yaml:"url"` // 支持 {domain} {ip} 占位符
	} `yaml:"callback"`

	HurricaneElectric struct {
		AccountID  string `yaml:"account_id"`
		ClientID   string `yaml:"client_id"`
		Password   string `yaml:"password"`
		NetworkKey string `yaml:"network_key"`
	} `yaml:"he"`

	NoIP struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"noip"`

	DuckDNS struct {
		Token string `yaml:"token"`
	} `yaml:"duckdns"`
}

// LoadCredentials 从配置目录加载认证凭据。
func LoadCredentials(configDir string) *Credentials {
	creds := &Credentials{}
	data, err := os.ReadFile(configDir + "/secrets.yaml")
	if err != nil {
		data, err = os.ReadFile(configDir + "/secrets.yml")
		if err != nil {
			// 文件不存在或读取失败，返回空凭据
			return creds
		}
	}
	if err := yaml.Unmarshal(data, creds); err != nil {
		// 解析失败，返回已解析部分
		return creds
	}
	return creds
}

// SaveCredentials 保存认证凭据到配置目录（权限 0600）。
func SaveCredentials(configDir string, c *Credentials) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(configDir+"/secrets.yaml", data, 0600)
}

// ============================================================================
// 阿里云 DNS
// ============================================================================

// AliDNSProvider 阿里云 DNS 服务商。
// 支持子账号 AK/SK，使用阿里云签名算法 v1。
type AliDNSProvider struct {
	creds    *Credentials
	client   *http.Client
	endpoint string
}

func NewAliDNSProvider() *AliDNSProvider {
	return &AliDNSProvider{
		creds:    &Credentials{},
		client:   &http.Client{Timeout: 30 * time.Second},
		endpoint: "https://alidns.aliyuncs.com",
	}
}

// WithCredentials 注入凭据。
func (p *AliDNSProvider) WithCredentials(c *Credentials) {
	p.creds = c
}

func (p *AliDNSProvider) Name() string { return "alidns" }

// UpdateIP 更新阿里云 DNS 记录。
func (p *AliDNSProvider) UpdateIP(domain, ip string) error {
	if p.creds.AliDNS.AccessKeyID == "" {
		return fmt.Errorf("阿里云 DNS 未配置 AccessKey")
	}
	params := map[string]string{
		"DomainName": domain,
		"RecordId":   "", // 需要查询 RecordId
		"RR":         "",
		"Type":       "A",
		"TTL":        "600",
		"Value":      ip,
	}

	// 先查询 RecordId
	recordID, err := p.alidnsDescribeRecord(domain)
	if err != nil {
		return err
	}
	params["RecordId"] = recordID

	// 构建请求并签名
	req, err := p.alidnsSignRequest("UpdateRecord", params)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("阿里云 API 错误 (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// alidnsDescribeRecord 查询域名对应的 RecordId。
func (p *AliDNSProvider) alidnsDescribeRecord(domain string) (string, error) {
	params := map[string]string{
		"DomainName": domain,
		"PageNumber": "1",
		"PageSize":   "20",
	}
	req, err := p.alidnsSignRequest("DescribeDomainRecords", params)
	if err != nil {
		return "", err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// 简化解析：实际需要 JSON 解析
	return "", fmt.Errorf("阿里云 DNS 记录解析待实现: %s", string(body))
}

// alidnsSignRequest 构建阿里云签名请求。
func (p *AliDNSProvider) alidnsSignRequest(action string, params map[string]string) (*http.Request, error) {
	form := url.Values{}
	form.Set("Action", action)
	form.Set("Version", "2019-04-27")
	form.Set("Format", "JSON")
	form.Set("AccessKeyId", p.creds.AliDNS.AccessKeyID)
	form.Set("SignatureMethod", "HMAC-SHA1")
	form.Set("SignatureVersion", "1.0")
	form.Set("SignatureNonce", fmt.Sprintf("%d", time.Now().UnixNano()))
	form.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	for k, v := range params {
		form.Set(k, v)
	}

	// 构建规范化查询串
	keys := make([]string, 0, len(params)+7)
	for k := range form {
		keys = append(keys, k)
	}
	// 简化：实际按阿里云规范排序
	_ = keys

	signature, err := alidnsHmacSHA1(p.creds.AliDNS.AccessKeySecret, form.Encode())
	if err != nil {
		return nil, err
	}
	form.Set("Signature", signature)

	req, err := http.NewRequest(http.MethodPost, p.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

// alidnsHmacSHA1 计算阿里云 HMAC-SHA1 签名。
func alidnsHmacSHA1(secret, message string) (string, error) {
	mac := hmac.New(sha1.New, []byte(secret))
	if _, err := mac.Write([]byte(message)); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// GetCurrentIP 获取域名当前解析 IP。
func (p *AliDNSProvider) GetCurrentIP(domain string) (string, error) {
	// 实际应通过 DNS 查询获取
	return "", fmt.Errorf("阿里云 DNS 当前 IP 查询待实现")
}

// ============================================================================
// DNSPod
// ============================================================================

// DNSPodProvider DNSPod 服务商。
type DNSPodProvider struct {
	creds  *Credentials
	client *http.Client
}

func NewDNSPodProvider() *DNSPodProvider {
	return &DNSPodProvider{
		creds:  &Credentials{},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *DNSPodProvider) WithCredentials(c *Credentials) { p.creds = c }
func (p *DNSPodProvider) Name() string                     { return "dnspod" }

// UpdateIP 更新 DNSPod 记录。
func (p *DNSPodProvider) UpdateIP(domain, ip string) error {
	if p.creds.DNSPod.APIKey == "" {
		return fmt.Errorf("DNSPod 未配置 API Key")
	}
	// 实际调用 https://dnsapi.cn/Record.modify
	return fmt.Errorf("DNSPod API 调用待实现")
}

func (p *DNSPodProvider) GetCurrentIP(domain string) (string, error) {
	return "", fmt.Errorf("DNSPod 当前 IP 查询待实现")
}

// ============================================================================
// Cloudflare
// ============================================================================

// CloudflareProvider Cloudflare 服务商。
type CloudflareProvider struct {
	creds  *Credentials
	client *http.Client
}

func NewCloudflareProvider() *CloudflareProvider {
	return &CloudflareProvider{
		creds:  &Credentials{},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *CloudflareProvider) WithCredentials(c *Credentials) { p.creds = c }
func (p *CloudflareProvider) Name() string                     { return "cloudflare" }

// UpdateIP 更新 Cloudflare 记录。
func (p *CloudflareProvider) UpdateIP(domain, ip string) error {
	if p.creds.Cloudflare.APIToken == "" {
		return fmt.Errorf("Cloudflare 未配置 API Token")
	}
	// 实际调用 https://api.cloudflare.com/client/v4/zones/{zone}/dns_records/{record}
	return fmt.Errorf("Cloudflare API 调用待实现")
}

func (p *CloudflareProvider) GetCurrentIP(domain string) (string, error) {
	return "", fmt.Errorf("Cloudflare 当前 IP 查询待实现")
}

// ============================================================================
// 自定义回调
// ============================================================================

// CallbackProvider 自定义 HTTP 回调。
type CallbackProvider struct {
	creds  *Credentials
	client *http.Client
}

func NewCallbackProvider() *CallbackProvider {
	return &CallbackProvider{
		creds:  &Credentials{},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *CallbackProvider) WithCredentials(c *Credentials) { p.creds = c }
func (p *CallbackProvider) Name() string                     { return "callback" }

// UpdateIP 执行自定义回调。
func (p *CallbackProvider) UpdateIP(domain, ip string) error {
	if p.creds.Callback.URL == "" {
		return fmt.Errorf("回调 URL 未配置")
	}
	// 替换占位符
	target := p.creds.Callback.URL
	target = strings.ReplaceAll(target, "{domain}", domain)
	target = strings.ReplaceAll(target, "{ip}", ip)

	resp, err := p.client.Get(target)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("回调返回 %d", resp.StatusCode)
	}
	return nil
}

func (p *CallbackProvider) GetCurrentIP(domain string) (string, error) {
	return "", nil
}

// ============================================================================
// Hurricane Electric
// ============================================================================

// HurricaneElectricProvider Hurricane Electric DDNS。
type HurricaneElectricProvider struct {
	creds  *Credentials
	client *http.Client
}

func NewHurricaneElectricProvider() *HurricaneElectricProvider {
	return &HurricaneElectricProvider{
		creds:  &Credentials{},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *HurricaneElectricProvider) WithCredentials(c *Credentials) { p.creds = c }
func (p *HurricaneElectricProvider) Name() string                     { return "he" }

// UpdateIP 更新 HE DDNS 记录。
func (p *HurricaneElectricProvider) UpdateIP(domain, ip string) error {
	if p.creds.HurricaneElectric.AccountID == "" {
		return fmt.Errorf("HE 未配置账号信息")
	}
	// 实际调用 https://api.he.net/ip/modify
	return fmt.Errorf("HE API 调用待实现")
}

func (p *HurricaneElectricProvider) GetCurrentIP(domain string) (string, error) {
	return "", nil
}

// ============================================================================
// No-IP
// ============================================================================

// NoIPProvider No-IP 服务商。
type NoIPProvider struct {
	creds  *Credentials
	client *http.Client
}

func NewNoIPProvider() *NoIPProvider {
	return &NoIPProvider{
		creds:  &Credentials{},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *NoIPProvider) WithCredentials(c *Credentials) { p.creds = c }
func (p *NoIPProvider) Name() string                     { return "noip" }

// UpdateIP 更新 No-IP 记录。
func (p *NoIPProvider) UpdateIP(domain, ip string) error {
	if p.creds.NoIP.Username == "" {
		return fmt.Errorf("No-IP 未配置账号")
	}
	// 实际调用 https://dynupdate.noip.com/ni1
	target := fmt.Sprintf("https://dynupdate.noip.com/ni1?hostname=%s&myip=%s",
		url.QueryEscape(domain), url.QueryEscape(ip))
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.creds.NoIP.Username, p.creds.NoIP.Password)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("No-IP 返回 %d", resp.StatusCode)
	}
	return nil
}

func (p *NoIPProvider) GetCurrentIP(domain string) (string, error) {
	return "", nil
}

// ============================================================================
// DuckDNS
// ============================================================================

// DuckDNSProvider DuckDNS 服务商。
type DuckDNSProvider struct {
	creds  *Credentials
	client *http.Client
}

func NewDuckDNSProvider() *DuckDNSProvider {
	return &DuckDNSProvider{
		creds:  &Credentials{},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *DuckDNSProvider) WithCredentials(c *Credentials) { p.creds = c }
func (p *DuckDNSProvider) Name() string                     { return "duckdns" }

// UpdateIP 更新 DuckDNS 记录。
func (p *DuckDNSProvider) UpdateIP(domain, ip string) error {
	// DuckDNS 支持 GET https://www.duckdns.org/update?domains=xxx&host=xxx&password=xxx
	target := fmt.Sprintf("https://www.duckdns.org/update?domains=%s&host=%s&password=%s",
		url.QueryEscape(domain), url.QueryEscape(domain), url.QueryEscape(p.creds.DuckDNS.Token))
	resp, err := p.client.Get(target)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "OK") {
		return fmt.Errorf("DuckDNS 更新失败: %s", string(body))
	}
	return nil
}

func (p *DuckDNSProvider) GetCurrentIP(domain string) (string, error) {
	return "", nil
}
