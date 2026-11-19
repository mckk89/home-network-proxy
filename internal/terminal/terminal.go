package terminal

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
)

// Manager Web 终端管理器。
// 通过 WebSocket 提供交互式 shell。
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*exec.Cmd
}

// NewManager 创建终端管理器。
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*exec.Cmd),
	}
}

// Upgrade 处理 WebSocket 升级请求。
func (m *Manager) Upgrade(c *websocket.Conn, sessionID string) {
	// 创建 shell 进程
	cmd := exec.Command("sh")
	stdinPipe, err1 := cmd.StdinPipe()
	stdoutPipe, err2 := cmd.StdoutPipe()
	if err1 != nil || err2 != nil {
		c.WriteMessage(websocket.TextMessage, []byte("终端启动失败\n"))
		c.Close()
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("终端启动失败: %v", err)
		c.WriteMessage(websocket.TextMessage, []byte("终端启动失败: "+err.Error()+"\n"))
		c.Close()
		return
	}

	m.mu.Lock()
	m.sessions[sessionID] = cmd
	m.mu.Unlock()

	// 读取 shell 输出
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				c.WriteMessage(websocket.TextMessage, buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// 处理客户端输入
	for {
		msgType, data, err := c.ReadMessage()
		if err != nil {
			break
		}
		if msgType == websocket.TextMessage {
			stdinPipe.Write(data)
		}
	}

	cmd.Wait()
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
}

// Start 启动管理器（空操作，会话按需创建）。
func (m *Manager) Start(ctx context.Context) error {
	log.Println("Web 终端管理器已就绪")
	return nil
}

// Stop 停止管理器。
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cmd := range m.sessions {
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Kill()
		}
		delete(m.sessions, id)
	}
	return nil
}

// 防止未使用
var _ = fmt.Sprintf

