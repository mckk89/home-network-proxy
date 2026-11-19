package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Manager Docker 管理模块。
// 通过 Docker Engine API 管理容器。
type Manager struct {
	socket string
	mu     sync.RWMutex
	client *http.Client
	running bool
}

// NewManager 创建 Docker 管理器。
// socket 为 Docker socket 路径（如 /var/run/docker.sock）。
func NewManager(socket string) *Manager {
	return &Manager{
		socket: socket,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Start 启动管理器。
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	log.Printf("Docker 管理器已启动 (socket=%s)", m.socket)
	return nil
}

// Stop 停止。
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

// Container 容器信息。
type Container struct {
	ID     string   `json:"id"`
	Names  []string `json:"names"`
	Image  string   `json:"image"`
	State  string   `json:"state"`
	Status  string   `json:"status"`
	Ports  []any    `json:"ports"`
}

// ListContainers 列出所有容器。
func (m *Manager) ListContainers() ([]Container, error) {
	resp, err := m.client.Get(m.dockerURL("/containers/json"))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var containers []Container
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, err
	}
	return containers, nil
}

// StartContainer 启动容器。
func (m *Manager) StartContainer(id string) error {
	_, err := m.client.Post(m.dockerURL(fmt.Sprintf("/containers/%s/start", id)), "application/json", nil)
	return err
}

// StopContainer 停止容器。
func (m *Manager) StopContainer(id string) error {
	_, err := m.client.Post(m.dockerURL(fmt.Sprintf("/containers/%s/stop", id)), "application/json", nil)
	return err
}

// RestartContainer 重启容器。
func (m *Manager) RestartContainer(id string) error {
	_, err := m.client.Post(m.dockerURL(fmt.Sprintf("/containers/%s/restart", id)), "application/json", nil)
	return err
}

// RemoveContainer 删除容器。
func (m *Manager) RemoveContainer(id string) error {
	req, err := http.NewRequest(http.MethodDelete, m.dockerURL(fmt.Sprintf("/containers/%s", id)), nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// dockerURL 构造 Docker API URL。
func (m *Manager) dockerURL(path string) string {
	// 简化：实际应使用 HTTP client 通过 unix socket 通信
	return fmt.Sprintf("http://localhost%s", path)
}

// Status 返回状态。
func (m *Manager) Status() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]interface{}{
		"running": m.running,
		"socket":  m.socket,
	}
}
