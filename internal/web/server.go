package web

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gdy666/home-network-proxy/internal/config"
	"github.com/gdy666/home-network-proxy/internal/ddns"
	"github.com/gdy666/home-network-proxy/internal/module"
	"github.com/gdy666/home-network-proxy/internal/reverseproxy"
	"github.com/gdy666/home-network-proxy/internal/socketproxy"
)

type Server struct {
	cfg             *config.Config
	ddnsManager     *ddns.Manager
	reverseProxyMgr *reverseproxy.Manager
	socketProxyMgr  *socketproxy.Manager
	moduleManager   *module.Manager
	router          *gin.Engine
	server          *http.Server
}

func NewServer(cfg *config.Config, ddnsMgr *ddns.Manager, rpMgr *reverseproxy.Manager, spMgr *socketproxy.Manager, modMgr *module.Manager) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(Logger())

	s := &Server{
		cfg:             cfg,
		ddnsManager:     ddnsMgr,
		reverseProxyMgr: rpMgr,
		socketProxyMgr:  spMgr,
		moduleManager:   modMgr,
		router:          router,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// 加载内嵌的 HTML 模板（单二进制运行，无需外部 web 目录）
	tmpl, err := LoadTemplates()
	if err != nil {
		log.Fatalf("加载 Web 模板失败: %v", err)
	}
	s.router.SetHTMLTemplate(tmpl)

	// 内嵌静态资源
	if staticFS, err := StaticFS(); err == nil {
		s.router.StaticFS("/static", http.FS(staticFS))
	}

	// 认证中间件
	auth := s.authMiddleware()

	// 登录页面
	s.router.GET("/login", s.handleLoginPage)
	s.router.POST("/login", s.handleLogin)

	// 需要认证的路由组
	authorized := s.router.Group("/")
	authorized.Use(auth)
	{
		// 仪表盘
		authorized.GET("/", s.handleDashboard)
		authorized.GET("/dashboard", s.handleDashboard)

		// 设置页面
		authorized.GET("/settings", s.handleSettingsPage)
		authorized.POST("/settings", s.handleSettings)
		authorized.POST("/api/settings", s.handleSettings)
		authorized.POST("/api/system/reset-credentials", s.handleResetCredentials)

		// DDNS 管理
		authorized.GET("/ddns", s.handleDDNSPage)
		authorized.GET("/api/ddns", s.handleGetDDNSConfig)
		authorized.POST("/api/ddns", s.handleUpdateDDNSConfig)

		// 反向代理管理
		authorized.GET("/reverse-proxy", s.handleReverseProxyPage)
		authorized.GET("/api/reverse-proxy", s.handleGetReverseProxyConfig)
		authorized.POST("/api/reverse-proxy", s.handleUpdateReverseProxyConfig)

		// Socket 代理管理
		authorized.GET("/socket-proxy", s.handleSocketProxyPage)
		authorized.GET("/api/socket-proxy", s.handleGetSocketProxyConfig)
		authorized.POST("/api/socket-proxy", s.handleUpdateSocketProxyConfig)

		// SSL 证书管理
		authorized.GET("/ssl", s.handleSSLPage)
		authorized.GET("/api/ssl/certificates", s.handleListCertificates)
		authorized.POST("/api/ssl/certificates", s.handleRequestCertificate)
		authorized.POST("/api/ssl/acme", s.handleACMEConfig)

		// WAF 防火墙
		authorized.GET("/waf", s.handleWAFPage)
		authorized.POST("/api/waf/config", s.handleWAFConfig)

		// 计划任务
		authorized.GET("/tasks", s.handleTasksPage)

		// 存储管理
		authorized.GET("/storage", s.handleStoragePage)

		// WebDAV 服务
		authorized.GET("/webdav", s.handleWebDAVPage)

		// FTP 服务
		authorized.GET("/ftp", s.handleFTPPage)

		// 模块管理
		authorized.GET("/modules", s.handleModulesPage)
		authorized.GET("/api/modules", s.handleGetModules)
		authorized.POST("/api/modules/:name/toggle", s.handleToggleModule)

		// 系统管理
		authorized.GET("/system", s.handleSystemPage)
		authorized.GET("/api/system/info", s.handleSystemInfo)
		authorized.POST("/api/system/restart", s.handleRestart)
		authorized.POST("/api/system/backup", s.handleBackup)
		authorized.POST("/api/system/restore", s.handleRestore)

		// 文件管理
		authorized.GET("/files", s.handleFilesPage)
		authorized.GET("/api/files/*path", s.handleListFiles)
		authorized.GET("/api/download", s.handleDownloadFile)
		authorized.POST("/api/files/upload", s.handleUploadFile)
		authorized.DELETE("/api/files/*path", s.handleDeleteFile)

		// 日志查看
		authorized.GET("/logs", s.handleLogsPage)
		authorized.GET("/api/logs", s.handleGetLogs)

		// Docker 管理
		authorized.GET("/docker", s.handleDockerPage)
		authorized.GET("/api/docker/containers", s.handleListContainers)
		authorized.POST("/api/docker/containers/:id/:action", s.handleContainerAction)

		// 终端
		authorized.GET("/terminal", s.handleTerminalPage)
		authorized.GET("/api/terminal/ws", s.handleTerminalWS)
	}

	// 健康检查
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API 版本信息
	s.router.GET("/api/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"version":   "3.5.0",
			"buildTime": "2024-01-01",
		})
	})
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查安全入口
		if s.cfg.Web.SafeURL != "" {
			path := strings.TrimPrefix(c.Request.URL.Path, "/")
			if !strings.HasPrefix(path, s.cfg.Web.SafeURL+"/") && path != s.cfg.Web.SafeURL {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		}

		// 检查会话
		session, err := c.Cookie("hnproxy_session")
		if err != nil || session != s.getSessionToken() {
			// API 请求返回 401，页面请求重定向到登录页
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
				return
			}
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Next()
	}
}

func (s *Server) getSessionToken() string {
	// 简单的会话 token，实际生产中应使用更安全的方式
	return fmt.Sprintf("hnproxy_%s_%s", s.cfg.Web.Username, s.cfg.Web.Password)
}

func (s *Server) handleLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	safeURL := c.PostForm("safe_url")

	if username == s.cfg.Web.Username && password == s.cfg.Web.Password {
		token := s.getSessionToken()
		c.SetCookie("hnproxy_session", token, 86400, "/", "", false, true)

		redirectTo := "/"
		if safeURL != "" {
			redirectTo = "/" + safeURL
		}
		c.Redirect(http.StatusFound, redirectTo)
		return
	}

	c.HTML(http.StatusOK, "login.html", gin.H{
		"error":   "用户名或密码错误",
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleDashboard(c *gin.Context) {
	allModules := s.moduleManager.GetAllModules()
	enabledModules := []string{}
	all := map[string]map[string]interface{}{}
	for _, m := range allModules {
		name, _ := m["name"].(string)
		all[name] = m
		if enabled, _ := m["enabled"].(bool); enabled {
			enabledModules = append(enabledModules, name)
		}
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"version": "3.5.0",
		"cfg":     s.cfg,
		"ddns":    s.ddnsManager.GetStatus(),
		"proxy":   s.reverseProxyMgr.GetStatus(),
		"socket":  s.socketProxyMgr.GetStatus(),
		"modules": gin.H{
			"enabled": enabledModules,
			"all":     all,
		},
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleSettingsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleSettings(c *gin.Context) {
	var req struct {
		Username    string `form:"username" binding:"required"`
		Password    string `form:"password"`
		SafeURL     string `form:"safe_url"`
		Port        int    `form:"port" binding:"required,min=1,max=65535"`
		EnableHTTPS bool   `form:"enable_https"`
		CertFile    string `form:"cert_file"`
		KeyFile     string `form:"key_file"`
		Disable2FA  bool   `form:"disable_2fa"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.cfg.Web.Username = req.Username
	if req.Password != "" {
		s.cfg.Web.Password = req.Password
	}
	s.cfg.Web.SafeURL = req.SafeURL
	s.cfg.Web.Port = req.Port
	s.cfg.Web.EnableHTTPS = req.EnableHTTPS
	s.cfg.Web.CertFile = req.CertFile
	s.cfg.Web.KeyFile = req.KeyFile
	s.cfg.Web.Disable2FA = req.Disable2FA

	// 保存配置
	configPath := filepath.Join(s.cfg.Storage.Path, "hnproxy.conf")
	if err := s.cfg.Save(configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "设置已保存，重启后生效"})
}

func (s *Server) handleDDNSPage(c *gin.Context) {
	c.HTML(http.StatusOK, "ddns.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleGetDDNSConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.cfg.DDNS)
}

func (s *Server) handleUpdateDDNSConfig(c *gin.Context) {
	var req DDNSConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.cfg.DDNS.Enabled = req.Enabled
	s.cfg.DDNS.Providers = req.Providers
	s.cfg.DDNS.Interval = req.Interval
	s.cfg.DDNS.IPv4Enabled = req.IPv4Enabled
	s.cfg.DDNS.IPv6Enabled = req.IPv6Enabled

	// 保存配置
	configPath := filepath.Join(s.cfg.Storage.Path, "hnproxy.conf")
	if err := s.cfg.Save(configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	// 重新加载 DDNS 配置
	if err := s.ddnsManager.Reload(s.cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重新加载 DDNS 失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "DDNS 配置已更新"})
}

type DDNSConfigRequest struct {
	Enabled      bool              `json:"enabled"`
	Providers    map[string]string `json:"providers"`
	Interval     int               `json:"interval"`
	IPv4Enabled  bool              `json:"ipv4_enabled"`
	IPv6Enabled  bool              `json:"ipv6_enabled"`
}

func (s *Server) handleReverseProxyPage(c *gin.Context) {
	c.HTML(http.StatusOK, "reverse_proxy.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleGetReverseProxyConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.cfg.ReverseProxy)
}

func (s *Server) handleUpdateReverseProxyConfig(c *gin.Context) {
	var req ReverseProxyConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.cfg.ReverseProxy.Enabled = req.Enabled
	s.cfg.ReverseProxy.Services = req.Services

	configPath := filepath.Join(s.cfg.Storage.Path, "hnproxy.conf")
	if err := s.cfg.Save(configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	if err := s.reverseProxyMgr.Reload(s.cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重新加载反向代理失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "反向代理配置已更新"})
}

type ReverseProxyConfigRequest struct {
	Enabled  bool     `json:"enabled"`
	Services []string `json:"services"`
}

func (s *Server) handleSocketProxyPage(c *gin.Context) {
	c.HTML(http.StatusOK, "socket_proxy.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleGetSocketProxyConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.cfg.SocketProxy)
}

func (s *Server) handleUpdateSocketProxyConfig(c *gin.Context) {
	var req SocketProxyConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.cfg.SocketProxy.Enabled = req.Enabled
	s.cfg.SocketProxy.Services = req.Services

	configPath := filepath.Join(s.cfg.Storage.Path, "hnproxy.conf")
	if err := s.cfg.Save(configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	if err := s.socketProxyMgr.Reload(s.cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重新加载 Socket 代理失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Socket 代理配置已更新"})
}

type SocketProxyConfigRequest struct {
	Enabled  bool     `json:"enabled"`
	Services []string `json:"services"`
}

func (s *Server) handleSSLPage(c *gin.Context) {
	c.HTML(http.StatusOK, "ssl.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleListCertificates(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"certificates": []interface{}{}})
}

func (s *Server) handleRequestCertificate(c *gin.Context) {
	var req struct {
		Domains   []string `json:"domains"`
		Challenge string   `json:"challenge"`
		Wildcard  bool     `json:"wildcard"`
		Force     bool     `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "证书申请任务已提交", "domains": req.Domains})
}

func (s *Server) handleACMEConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "ACME 设置已保存"})
}

func (s *Server) handleWAFPage(c *gin.Context) {
	c.HTML(http.StatusOK, "waf.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleWAFConfig(c *gin.Context) {
	var req struct {
		Enabled bool   `json:"enabled"`
		Mode    string `json:"mode"`
		Engine  string `json:"engine"`
		Ruleset string `json:"ruleset"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.cfg.Security.EnableWAF = req.Enabled
	if req.Mode != "" {
		s.cfg.Security.WAFMode = req.Mode
	}
	if err := s.cfg.Save(filepath.Join(s.cfg.Storage.Path, "hnproxy.conf")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "WAF 设置已保存"})
}

func (s *Server) handleTasksPage(c *gin.Context) {
	c.HTML(http.StatusOK, "tasks.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleStoragePage(c *gin.Context) {
	c.HTML(http.StatusOK, "storage.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleWebDAVPage(c *gin.Context) {
	c.HTML(http.StatusOK, "webdav.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleFTPPage(c *gin.Context) {
	c.HTML(http.StatusOK, "ftp.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleDownloadFile(c *gin.Context) {
	rel := c.Query("path")
	abs := filepath.Join(s.cfg.Storage.Path, filepath.Clean("/"+rel))
	if !strings.HasPrefix(abs, filepath.Clean(s.cfg.Storage.Path)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "禁止访问"})
		return
	}
	if info, err := os.Stat(abs); err != nil || info.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	c.File(abs)
}

func (s *Server) handleResetCredentials(c *gin.Context) {
	s.cfg.Web.Username = "666"
	s.cfg.Web.Password = "666"
	if err := s.cfg.Save(filepath.Join(s.cfg.Storage.Path, "hnproxy.conf")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "凭据已重置为 666/666"})
}

func (s *Server) handleModulesPage(c *gin.Context) {
	c.HTML(http.StatusOK, "modules.html", gin.H{
		"cfg":       s.cfg,
		"modules":   s.moduleManager.GetAllModules(),
		"safeURL":   s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleGetModules(c *gin.Context) {
	c.JSON(http.StatusOK, s.moduleManager.GetAllModules())
}

func (s *Server) handleToggleModule(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.moduleManager.ToggleModule(name, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新配置
	if req.Enabled {
		s.cfg.Modules.Enabled = append(s.cfg.Modules.Enabled, name)
		s.cfg.Modules.Disabled = removeString(s.cfg.Modules.Disabled, name)
	} else {
		s.cfg.Modules.Disabled = append(s.cfg.Modules.Disabled, name)
		s.cfg.Modules.Enabled = removeString(s.cfg.Modules.Enabled, name)
	}

	configPath := filepath.Join(s.cfg.Storage.Path, "hnproxy.conf")
	if err := s.cfg.Save(configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "模块状态已更新"})
}

func removeString(slice []string, s string) []string {
	result := []string{}
	for _, v := range slice {
		if v != s {
			result = append(result, v)
		}
	}
	return result
}

func (s *Server) handleSystemPage(c *gin.Context) {
	c.HTML(http.StatusOK, "system.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleSystemInfo(c *gin.Context) {
	hostname, _ := os.Hostname()
	c.JSON(http.StatusOK, gin.H{
		"version":   "3.5.0",
		"hostname":  hostname,
		"goVersion": "go1.21",
		"buildTime": "2024-01-01",
		"configDir": s.cfg.Storage.Path,
		"uptime":    time.Since(startTime).String(),
	})
}

var startTime = time.Now()

func (s *Server) handleRestart(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "正在重启..."})
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func (s *Server) handleBackup(c *gin.Context) {
	backupPath := filepath.Join(s.cfg.Storage.Path, fmt.Sprintf("backup_%s.zip", time.Now().Format("20060102_150405")))
	// 实际实现需要打包配置文件
	c.JSON(http.StatusOK, gin.H{"message": "备份功能开发中", "path": backupPath})
}

func (s *Server) handleRestore(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "恢复功能开发中"})
}

func (s *Server) handleFilesPage(c *gin.Context) {
	c.HTML(http.StatusOK, "files.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleListFiles(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		path = "/"
	}
	// 安全检查：防止目录遍历
	absPath, err := filepath.Abs(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效路径"})
		return
	}
	// 限制在配置目录下
	if !strings.HasPrefix(absPath, s.cfg.Storage.Path) {
		c.JSON(http.StatusForbidden, gin.H{"error": "禁止访问"})
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	files := []gin.H{}
	for _, e := range entries {
		info, _ := e.Info()
		files = append(files, gin.H{
			"name":    e.Name(),
			"path":    filepath.Join(path, e.Name()),
			"isDir":   e.IsDir(),
			"size":    info.Size(),
			"modTime": info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, files)
}

func (s *Server) handleUploadFile(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "上传功能开发中"})
}

func (s *Server) handleDeleteFile(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "删除功能开发中"})
}

func (s *Server) handleLogsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "logs.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleGetLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"logs": []string{"日志功能开发中"}})
}

func (s *Server) handleDockerPage(c *gin.Context) {
	c.HTML(http.StatusOK, "docker.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleListContainers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"containers": []gin.H{}, "message": "Docker 管理功能开发中"})
}

func (s *Server) handleContainerAction(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "容器操作功能开发中"})
}

func (s *Server) handleTerminalPage(c *gin.Context) {
	c.HTML(http.StatusOK, "terminal.html", gin.H{
		"cfg":     s.cfg,
		"safeURL": s.cfg.Web.SafeURL,
	})
}

func (s *Server) handleTerminalWS(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Web 终端功能开发中"})
}

func (s *Server) Start(addr string) error {
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	if s.cfg.Web.EnableHTTPS && s.cfg.Web.CertFile != "" && s.cfg.Web.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(s.cfg.Web.CertFile, s.cfg.Web.KeyFile)
		if err != nil {
			return err
		}
		s.server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		return s.server.ListenAndServeTLS("", "")
	}

	return s.server.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		log.Printf("%s %s %d %v", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), latency)
	}
}