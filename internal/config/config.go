package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 主配置结构
type Config struct {
	Web       WebConfig       `yaml:"web" json:"web"`
	DDNS      DDNSConfig      `yaml:"ddns" json:"ddns"`
	ReverseProxy ReverseProxyConfig `yaml:"reverse_proxy" json:"reverse_proxy"`
	SocketProxy  SocketProxyConfig  `yaml:"socket_proxy" json:"socket_proxy"`
	Modules   ModulesConfig   `yaml:"modules" json:"modules"`
	Storage   StorageConfig   `yaml:"storage" json:"storage"`
	Security  SecurityConfig  `yaml:"security" json:"security"`
}

// WebConfig Web 管理界面配置
type WebConfig struct {
	Port         int    `yaml:"port" json:"port" default:"16601"`
	Username     string `yaml:"username" json:"username" default:"666"`
	Password     string `yaml:"password" json:"password" default:"666"`
	SafeURL      string `yaml:"safe_url" json:"safe_url" default:""`
	EnableHTTPS  bool   `yaml:"enable_https" json:"enable_https" default:"false"`
	CertFile     string `yaml:"cert_file" json:"cert_file" default:""`
	KeyFile      string `yaml:"key_file" json:"key_file" default:""`
	Disable2FA   bool   `yaml:"disable_2fa" json:"disable_2fa" default:"false"`
}

// DDNSConfig DDNS 配置
type DDNSConfig struct {
	Enabled    bool              `yaml:"enabled" json:"enabled" default:"false"`
	Providers  map[string]string `yaml:"providers" json:"providers"`
	Interval   int               `yaml:"interval" json:"interval" default:"300"`
	IPv4Enabled bool             `yaml:"ipv4_enabled" json:"ipv4_enabled" default:"true"`
	IPv6Enabled bool             `yaml:"ipv6_enabled" json:"ipv6_enabled" default:"true"`
}

// ReverseProxyConfig 反向代理配置
type ReverseProxyConfig struct {
	Enabled  bool     `yaml:"enabled" json:"enabled" default:"false"`
	Services []string `yaml:"services" json:"services"`
}

// SocketProxyConfig Socket 代理配置
type SocketProxyConfig struct {
	Enabled  bool     `yaml:"enabled" json:"enabled" default:"false"`
	Services []string `yaml:"services" json:"services"`
}

// ModulesConfig 模块配置
type ModulesConfig struct {
	Enabled []string `yaml:"enabled" json:"enabled"`
	Disabled []string `yaml:"disabled" json:"disabled"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Path string `yaml:"path" json:"path" default:"/app/conf"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	IPWhitelist  []string `yaml:"ip_whitelist" json:"ip_whitelist"`
	IPBlacklist  []string `yaml:"ip_blacklist" json:"ip_blacklist"`
	EnableWAF    bool     `yaml:"enable_waf" json:"enable_waf" default:"false"`
	WAFMode      string   `yaml:"waf_mode" json:"waf_mode" default:"detection"`
}

// 默认配置
func DefaultConfig() *Config {
	return &Config{
		Web: WebConfig{
			Port:     16601,
			Username: "666",
			Password: "666",
			SafeURL:  "",
		},
		DDNS: DDNSConfig{
			Enabled:     false,
			Providers:   make(map[string]string),
			Interval:    300,
			IPv4Enabled: true,
			IPv6Enabled: true,
		},
		ReverseProxy: ReverseProxyConfig{
			Enabled:  false,
			Services: []string{},
		},
		SocketProxy: SocketProxyConfig{
			Enabled:  false,
			Services: []string{},
		},
		Modules: ModulesConfig{
			Enabled:  []string{},
			Disabled: []string{},
		},
		Storage: StorageConfig{
			Path: "/app/conf",
		},
		Security: SecurityConfig{
			IPWhitelist: []string{},
			IPBlacklist: []string{},
			EnableWAF:   false,
			WAFMode:     "detection",
		},
	}
}

// Load 加载配置文件
func Load(configFile, configDir string) (*Config, error) {
	cfg := DefaultConfig()

	// 确定配置文件路径
	var configPath string
	if configFile != "" {
		configPath = configFile
	} else if configDir != "" {
		configPath = filepath.Join(configDir, "hnproxy.conf")
	} else {
		// 尝试默认路径
		homeDir, _ := os.UserHomeDir()
		defaultPaths := []string{
			"/etc/hnproxy/hnproxy.conf",
			"/app/conf/hnproxy.conf",
			filepath.Join(homeDir, "hnproxy.conf"),
			"./hnproxy.conf",
		}
		for _, p := range defaultPaths {
			if _, err := os.Stat(p); err == nil {
				configPath = p
				break
			}
		}
	}

	// 如果找到配置文件，加载它
	if configPath != "" {
		if err := loadConfigFile(configPath, cfg); err != nil {
			return nil, err
		}
	}

	// 确保存储目录存在
	if err := os.MkdirAll(cfg.Storage.Path, 0755); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadConfigFile 从文件加载配置
func loadConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// 根据文件扩展名决定解析方式
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, cfg)
	case ".json":
		return json.Unmarshal(data, cfg)
	default:
		// 尝试 YAML 解析
		if err := yaml.Unmarshal(data, cfg); err != nil {
			// 尝试 JSON 解析
			return json.Unmarshal(data, cfg)
		}
		return nil
	}
}

// Save 保存配置到文件
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetConfigDir 获取配置目录
func (c *Config) GetConfigDir() string {
	return c.Storage.Path
}