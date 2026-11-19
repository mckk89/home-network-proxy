package tasks

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Manager 计划任务管理器。
// 支持 cron 表达式、固定间隔、单次执行。
type Manager struct {
	cron     *cron.Cron
	mu       sync.RWMutex
	entries  map[string]cron.EntryID
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  bool
}

// NewManager 创建任务管理器。
func NewManager() *Manager {
	c := cron.New(cron.WithSeconds())
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		cron:    c,
		entries: make(map[string]cron.EntryID),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start 启动任务管理器。
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}
	m.running = true
	m.cron.Start()
	log.Println("计划任务管理器已启动")
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
	ctx := m.cron.Stop()
	<-ctx.Done()
	m.running = false
	log.Println("计划任务管理器已停止")
	return nil
}

// AddCronTask 添加 cron 任务。
func (m *Manager) AddCronTask(id, spec string, fn func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entryID, err := m.cron.AddFunc(spec, fn)
	if err != nil {
		return err
	}
	m.entries[id] = entryID
	log.Printf("cron 任务 %s 已注册: %s", id, spec)
	return nil
}

// AddIntervalTask 添加固定间隔任务。
func (m *Manager) AddIntervalTask(id string, interval time.Duration, fn func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entryID, err := m.cron.AddFunc(
		"@every "+interval.String(),
		fn,
	)
	if err != nil {
		return err
	}
	m.entries[id] = entryID
	log.Printf("间隔任务 %s 已注册: 每 %s", id, interval)
	return nil
}

// RemoveTask 删除任务。
func (m *Manager) RemoveTask(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	entryID, ok := m.entries[id]
	if !ok {
		return false
	}
	m.cron.Remove(entryID)
	delete(m.entries, id)
	return true
}

// Status 返回状态。
func (m *Manager) Status() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]interface{}{
		"running": m.running,
		"tasks":   len(m.entries),
	}
}

// 防止未使用
var _ = time.Second
