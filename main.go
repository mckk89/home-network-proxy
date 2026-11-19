package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdy666/home-network-proxy/internal/config"
	"github.com/gdy666/home-network-proxy/internal/web"
	"github.com/gdy666/home-network-proxy/internal/ddns"
	"github.com/gdy666/home-network-proxy/internal/reverseproxy"
	"github.com/gdy666/home-network-proxy/internal/socketproxy"
	"github.com/gdy666/home-network-proxy/internal/module"
)

const (
	Version   = "3.5.0"
	BuildTime = "2024-01-01"
)

func main() {
	var (
		configFile = flag.String("c", "", "配置文件路径")
		configDir  = flag.String("cd", "", "配置目录路径")
		showVersion = flag.Bool("version", false, "显示版本信息")
		showInfo   = flag.Bool("info", false, "显示构建信息")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("家庭网络反代 %s\n", Version)
		os.Exit(0)
	}

	if *showInfo {
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("BuildTime: %s\n", BuildTime)
		fmt.Printf("GoVersion: %s\n", "go1.21")
		os.Exit(0)
	}

	// 初始化配置
	cfg, err := config.Load(*configFile, *configDir)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化各模块
	ddnsManager := ddns.NewManager(cfg)
	reverseProxyManager := reverseproxy.NewManager(cfg)
	socketProxyManager := socketproxy.NewManager(cfg)
	moduleManager := module.NewManager(cfg)

	// 启动 Web 管理界面
	webServer := web.NewServer(cfg, ddnsManager, reverseProxyManager, socketProxyManager, moduleManager)

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 HTTP 服务器
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Web.Port)
		log.Printf("启动 Web 管理界面: http://0.0.0.0%s", addr)
		if err := webServer.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Printf("Web 服务器错误: %v", err)
		}
	}()

	// 启动各模块
	if err := ddnsManager.Start(ctx); err != nil {
		log.Printf("DDNS 启动失败: %v", err)
	}
	if err := reverseProxyManager.Start(ctx); err != nil {
		log.Printf("反向代理启动失败: %v", err)
	}
	if err := socketProxyManager.Start(ctx); err != nil {
		log.Printf("Socket 代理启动失败: %v", err)
	}
	if err := moduleManager.Start(ctx); err != nil {
		log.Printf("模块管理器启动失败: %v", err)
	}

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("正在关闭服务...")
	cancel()

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := webServer.Stop(shutdownCtx); err != nil {
		log.Printf("Web 服务器关闭错误: %v", err)
	}
	ddnsManager.Stop()
	reverseProxyManager.Stop()
	socketProxyManager.Stop()
	moduleManager.Stop()

	log.Println("服务已关闭")
}