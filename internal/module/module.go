package module

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/gdy666/home-network-proxy/internal/config"
	"github.com/gdy666/home-network-proxy/internal/docker"
	"github.com/gdy666/home-network-proxy/internal/terminal"
	"github.com/gdy666/home-network-proxy/internal/tasks"
	"github.com/gdy666/home-network-proxy/internal/waf"
	"github.com/gdy666/home-network-proxy/internal/webdav"
	"github.com/gdy666/home-network-proxy/internal/wol"
)

// Module 模块接口
type Module interface {
	Name() string
	Description() string
	Start(ctx context.Context) error
	Stop() error
	Status() map[string]interface{}
	ConfigSchema() map[string]interface{}
}

// Manager 模块管理器
type Manager struct {
	cfg      *config.Config
	modules  map[string]Module
	enabled  map[string]bool
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  bool
}

// NewManager 创建模块管理器
func NewManager(cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		cfg:     cfg,
		modules: make(map[string]Module),
		enabled: make(map[string]bool),
		ctx:     ctx,
		cancel:  cancel,
	}
	m.registerModules()
	m.applyConfig()
	return m
}

// registerModules 注册内置模块
func (m *Manager) registerModules() {
	// 基础模块（所有版本都有）
	m.modules["ddns"] = NewDDNSModule()
	m.modules["reverse_proxy"] = NewReverseProxyModule()
	m.modules["socket_proxy"] = NewSocketProxyModule()
	m.modules["ssl"] = NewSSLModule()
	m.modules["tasks"] = NewTasksModule()
	m.modules["auth"] = NewAuthModule()
	m.modules["logs"] = NewLogsModule()
	m.modules["ipdb"] = NewIPDBModule()
	m.modules["icons"] = NewIconsModule()
	m.modules["webui"] = NewWebUIModule()

	// 存储相关模块
	m.modules["storage"] = NewStorageModule()
	m.modules["ftp"] = NewFTPModule()
	m.modules["webdav"] = NewWebDAVModule()

	// 网络工具
	m.modules["wol"] = NewWOLModule()
	m.modules["terminal"] = NewTerminalModule()
	m.modules["passkey"] = NewPasskeyModule()
	m.modules["homeid"] = NewHomeIDModule()

	// 安全模块
	m.modules["waf"] = NewWAFModule()
	m.modules["grpc_proxy"] = NewGRPCProxyModule()

	// 媒体/文件服务
	m.modules["dlna"] = NewDLNAModule()
	m.modules["cloudflare_tunnel"] = NewCloudflareTunnelModule()
	m.modules["frp"] = NewFRPModule()
	m.modules["smb"] = NewSMBModule()
	m.modules["filebrowser"] = NewFileBrowserModule()
	m.modules["rclone"] = NewRcloneModule()

	// Docker 管理
	m.modules["docker"] = NewDockerModule()
}

// applyConfig 应用配置中的启用/禁用状态
func (m *Manager) applyConfig() {
	for _, name := range m.cfg.Modules.Enabled {
		m.enabled[name] = true
	}
	for _, name := range m.cfg.Modules.Disabled {
		m.enabled[name] = false
	}
	// 默认启用基础模块
	defaultEnabled := []string{"ddns", "reverse_proxy", "socket_proxy", "ssl", "tasks", "auth", "logs", "ipdb", "icons", "webui"}
	for _, name := range defaultEnabled {
		if _, ok := m.enabled[name]; !ok {
			m.enabled[name] = true
		}
	}
}

// Start 启动所有启用的模块
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	for name, module := range m.modules {
		if !m.enabled[name] {
			continue
		}
		if err := module.Start(ctx); err != nil {
			log.Printf("启动模块 %s 失败: %v", name, err)
			continue
		}
		log.Printf("模块 %s 已启动", name)
	}

	m.running = true
	log.Println("模块管理器已启动")
	return nil
}

// Stop 停止所有模块
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	for name, module := range m.modules {
		if err := module.Stop(); err != nil {
			log.Printf("停止模块 %s 失败: %v", name, err)
		}
	}

	m.cancel()
	m.wg.Wait()
	m.running = false
	log.Println("模块管理器已停止")
}

// Reload 重新加载配置
func (m *Manager) Reload(cfg *config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cfg = cfg
	m.enabled = make(map[string]bool)
	m.applyConfig()

	// 重启变更的模块
	for name, module := range m.modules {
		if m.enabled[name] {
			if err := module.Start(m.ctx); err != nil {
				log.Printf("重启模块 %s 失败: %v", name, err)
			}
		} else {
			module.Stop()
		}
	}

	return nil
}

// ToggleModule 切换模块状态
func (m *Manager) ToggleModule(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	module, ok := m.modules[name]
	if !ok {
		return ErrModuleNotFound
	}

	if enabled == m.enabled[name] {
		return nil // 状态未变
	}

	if enabled {
		if err := module.Start(m.ctx); err != nil {
			return err
		}
		m.enabled[name] = true
	} else {
		if err := module.Stop(); err != nil {
			return err
		}
		m.enabled[name] = false
	}

	return nil
}

// GetStatus 获取所有模块状态
func (m *Manager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]interface{})
	for name, module := range m.modules {
		status[name] = map[string]interface{}{
			"enabled":  m.enabled[name],
			"running":  m.running && m.enabled[name],
			"status":   module.Status(),
		}
	}
	return status
}

// GetAllModules 获取所有模块信息
func (m *Manager) GetAllModules() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := []map[string]interface{}{}
	for name, module := range m.modules {
		result = append(result, map[string]interface{}{
			"name":        name,
			"description": module.Description(),
			"enabled":     m.enabled[name],
			"schema":      module.ConfigSchema(),
		})
	}
	return result
}

// ModuleNotFoundError 模块未找到错误
var ErrModuleNotFound = &ModuleError{Message: "模块不存在"}

type ModuleError struct {
	Message string
}

func (e *ModuleError) Error() string {
	return e.Message
}

// ==================== 内置模块实现 ====================

// BaseModule 基础模块结构
type BaseModule struct {
	name        string
	description string
	running     bool
}

func (b *BaseModule) Name() string        { return b.name }
func (b *BaseModule) Description() string { return b.description }
func (b *BaseModule) Status() map[string]interface{} {
	return map[string]interface{}{"running": b.running}
}
func (b *BaseModule) ConfigSchema() map[string]interface{} { return nil }
func (b *BaseModule) Start(ctx context.Context) error {
	b.running = true
	return nil
}
func (b *BaseModule) Stop() error {
	b.running = false
	return nil
}

// DDNSModule DDNS 模块
type DDNSModule struct{ BaseModule }

func NewDDNSModule() *DDNSModule {
	return &DDNSModule{BaseModule{name: "ddns", description: "动态域名解析"}}
}

// ReverseProxyModule 反向代理模块
type ReverseProxyModule struct{ BaseModule }

func NewReverseProxyModule() *ReverseProxyModule {
	return &ReverseProxyModule{BaseModule{name: "reverse_proxy", description: "Web 反向代理"}}
}

// SocketProxyModule Socket 代理模块
type SocketProxyModule struct{ BaseModule }

func NewSocketProxyModule() *SocketProxyModule {
	return &SocketProxyModule{BaseModule{name: "socket_proxy", description: "TCP/UDP 端口转发"}}
}

// SSLModule SSL 证书管理模块
type SSLModule struct{ BaseModule }

func NewSSLModule() *SSLModule {
	return &SSLModule{BaseModule{name: "ssl", description: "SSL/TLS 证书申请与续期"}}
}

// AuthModule 认证模块
type AuthModule struct{ BaseModule }

func NewAuthModule() *AuthModule {
	return &AuthModule{BaseModule{name: "auth", description: "第三方认证管理"}}
}

// LogsModule 日志中心模块
type LogsModule struct{ BaseModule }

func NewLogsModule() *LogsModule {
	return &LogsModule{BaseModule{name: "logs", description: "日志中心"}}
}

// IPDBModule IP 地址库模块
type IPDBModule struct{ BaseModule }

func NewIPDBModule() *IPDBModule {
	return &IPDBModule{BaseModule{name: "ipdb", description: "IP 地址库"}}
}

// IconsModule 图标库模块
type IconsModule struct{ BaseModule }

func NewIconsModule() *IconsModule {
	return &IconsModule{BaseModule{name: "icons", description: "图标库"}}
}

// WebUIModule Web 管理界面模块
type WebUIModule struct{ BaseModule }

func NewWebUIModule() *WebUIModule {
	return &WebUIModule{BaseModule{name: "webui", description: "内嵌 Web 管理界面"}}
}

// StorageModule 存储管理模块
type StorageModule struct{ BaseModule }

func NewStorageModule() *StorageModule {
	return &StorageModule{BaseModule{name: "storage", description: "存储管理"}}
}

// FTPModule FTP 服务模块
type FTPModule struct{ BaseModule }

func NewFTPModule() *FTPModule {
	return &FTPModule{BaseModule{name: "ftp", description: "FTP 服务"}}
}

// WebDAVModule WebDAV 服务模块
type WebDAVModule struct {
	BaseModule
	server *webdav.Server
}

func NewWebDAVModule() *WebDAVModule {
	return &WebDAVModule{BaseModule: BaseModule{name: "webdav", description: "WebDAV 服务"}}
}

// Start 启动 WebDAV 服务。
func (m *WebDAVModule) Start(ctx context.Context) error {
	root := "/app/conf/webdav"
	if dir, err := os.UserHomeDir(); err == nil {
		root = filepath.Join(dir, ".webdav")
	}
	m.server = webdav.NewServer(root, false)
	return m.server.Start(ctx, 16602)
}

// Stop 停止 WebDAV 服务。
func (m *WebDAVModule) Stop() error {
	if m.server == nil {
		return nil
	}
	return m.server.Stop()
}

// Status 返回 WebDAV 状态。
func (m *WebDAVModule) Status() map[string]interface{} {
	if m.server == nil {
		return map[string]interface{}{"running": false}
	}
	return m.server.Status()
}

// WOLModule 网络唤醒模块
type WOLModule struct {
	BaseModule
	*wol.Manager
}

func NewWOLModule() *WOLModule {
	return &WOLModule{
		BaseModule: BaseModule{name: "wol", description: "网络唤醒"},
		Manager:    wol.NewManager(),
	}
}

func (m *WOLModule) Status() map[string]interface{}{
	return map[string]interface{}{
		"running": m.running,
	}
}

func (m *WOLModule) Start(ctx context.Context) error {
	m.running = true
	return m.Manager.Start(ctx)
}

func (m *WOLModule) Stop() error {
	m.running = false
	return m.Manager.Stop()
}

// TerminalModule Web 终端模块
type TerminalModule struct {
	BaseModule
	*terminal.Manager
}

func NewTerminalModule() *TerminalModule {
	return &TerminalModule{
		BaseModule: BaseModule{name: "terminal", description: "Web 终端"},
		Manager:    terminal.NewManager(),
	}
}

func (m *TerminalModule) Status() map[string]interface{}{
	return map[string]interface{}{
		"running": m.running,
	}
}

func (m *TerminalModule) Start(ctx context.Context) error {
	m.running = true
	return m.Manager.Start(ctx)
}

func (m *TerminalModule) Stop() error {
	m.running = false
	return m.Manager.Stop()
}

// PasskeyModule Passkey 管理模块
type PasskeyModule struct{ BaseModule }

func NewPasskeyModule() *PasskeyModule {
	return &PasskeyModule{BaseModule{name: "passkey", description: "原生 Passkey 管理"}}
}

// HomeIDModule HomeID 模块
type HomeIDModule struct{ BaseModule }

func NewHomeIDModule() *HomeIDModule {
	return &HomeIDModule{BaseModule{name: "homeid", description: "HomeID"}}
}

// WAFModule Coraza WAF 模块
type WAFModule struct {
	BaseModule
	*waf.Manager
}

func NewWAFModule() *WAFModule {
	return &WAFModule{
		BaseModule: BaseModule{name: "waf", description: "Coraza WAF"},
		Manager:    waf.NewManager(),
	}
}

func (m *WAFModule) Status() map[string]interface{}{
	return map[string]interface{}{
		"running": m.running,
	}
}

func (m *WAFModule) Start(ctx context.Context) error {
	m.running = true
	return m.Manager.Start(ctx)
}

func (m *WAFModule) Stop() error {
	m.running = false
	return m.Manager.Stop()
}

// GRPCProxyModule gRPC 反向代理模块
type GRPCProxyModule struct{ BaseModule }

func NewGRPCProxyModule() *GRPCProxyModule {
	return &GRPCProxyModule{BaseModule{name: "grpc_proxy", description: "gRPC 反向代理"}}
}

// DLNAModule DLNA 服务模块
type DLNAModule struct{ BaseModule }

func NewDLNAModule() *DLNAModule {
	return &DLNAModule{BaseModule{name: "dlna", description: "DLNA 服务"}}
}

// CloudflareTunnelModule Cloudflare Tunnel 模块
type CloudflareTunnelModule struct{ BaseModule }

func NewCloudflareTunnelModule() *CloudflareTunnelModule {
	return &CloudflareTunnelModule{BaseModule{name: "cloudflare_tunnel", description: "Cloudflare Tunnel"}}
}

// FRPModule FRP 模块
type FRPModule struct{ BaseModule }

func NewFRPModule() *FRPModule {
	return &FRPModule{BaseModule{name: "frp", description: "FRP 内网穿透"}}
}

// SMBModule SMB 服务模块
type SMBModule struct{ BaseModule }

func NewSMBModule() *SMBModule {
	return &SMBModule{BaseModule{name: "smb", description: "SMB 服务"}}
}

// FileBrowserModule FileBrowser 模块
type FileBrowserModule struct{ BaseModule }

func NewFileBrowserModule() *FileBrowserModule {
	return &FileBrowserModule{BaseModule{name: "filebrowser", description: "FileBrowser"}}
}

// RcloneModule Rclone 模块
type RcloneModule struct{ BaseModule }

func NewRcloneModule() *RcloneModule {
	return &RcloneModule{BaseModule{name: "rclone", description: "Rclone"}}
}

// DockerModule Docker 管理模块
type DockerModule struct {
	BaseModule
	*docker.Manager
}

func NewDockerModule() *DockerModule {
	return &DockerModule{
		BaseModule: BaseModule{name: "docker", description: "Docker 管理"},
		Manager:    docker.NewManager("/var/run/docker.sock"),
	}
}

func (m *DockerModule) Start(ctx context.Context) error {
	m.running = true
	return m.Manager.Start(ctx)
}

func (m *DockerModule) Stop() error {
	m.running = false
	return m.Manager.Stop()
}

// TasksModule 计划任务模块
type TasksModule struct {
	BaseModule
	*tasks.Manager
}

func NewTasksModule() *TasksModule {
	return &TasksModule{
		BaseModule: BaseModule{name: "tasks", description: "计划任务"},
		Manager:    tasks.NewManager(),
	}
}

func (m *TasksModule) Status() map[string]interface{}{
	return map[string]interface{}{
		"running": m.running,
	}
}

func (m *DockerModule) Status() map[string]interface{}{
	return map[string]interface{}{
		"running": m.running,
	}
}

func (m *TasksModule) Start(ctx context.Context) error {
	m.running = true
	return m.Manager.Start(ctx)
}

func (m *TasksModule) Stop() error {
	m.running = false
	return m.Manager.Stop()
}

// TasksModule 计划任务模块（重复声明已移除）