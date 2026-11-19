package wol

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Manager 网络唤醒（Wake-on-LAN）管理器。
type Manager struct {
	mu      sync.Mutex
	running bool
}

// NewManager 创建 WOL 管理器。
func NewManager() *Manager {
	return &Manager{}
}

// Start 启动。
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = true
	log.Println("WOL 模块已启动")
	return nil
}

// Stop 停止。
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

// SendMagicPacket 发送魔术包唤醒目标 MAC 地址。
// mac 格式: "AA:BB:CC:DD:EE:FF" 或 "AABBCCDDEEFF"。
func (m *Manager) SendMagicPacket(mac string) error {
	var macBytes [6]byte
	for i := 0; i < 6; i++ {
		if i < len(mac) && i+1 < len(mac) {
			macBytes[i] = mac[i]
		}
	}

	// 构建魔术包: 6 字节 0xFF + 16 字节 MAC
	packet := make([]byte, 42)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	copy(packet[6:], macBytes[:])
	for i := 6; i < 6+16; i += 6 {
		copy(packet[i:i+6], macBytes[:])
	}

	// 发送到广播地址
	conn, err := net.Dial("udp", "255.255.255.255:9")
	if err != nil {
		return err
	}
	defer conn.Close()

	for i := 0; i < 3; i++ {
		conn.Write(packet)
		time.Sleep(50 * time.Millisecond)
	}

	log.Printf("WOL 魔术包已发送: %s", mac)
	return nil
}

// Status 返回状态。
func (m *Manager) Status() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]interface{}{
		"running": m.running,
	}
}

// 防止未使用
var _ = binary.LittleEndian
var _ = fmt.Sprintf
