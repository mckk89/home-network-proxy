package tasks

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.cron == nil {
		t.Fatal("cron not initialized")
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager()
	ctx := context.Background()
	
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !m.running {
		t.Error("expected running=true")
	}
	
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if m.running {
		t.Error("expected running=false")
	}
}

func TestAddCronTask(t *testing.T) {
	m := NewManager()
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()
	
	executed := 0
	if err := m.AddCronTask("test", "@every 1s", func() {
		executed++
	}); err != nil {
		t.Fatalf("AddCronTask failed: %v", err)
	}
	
	time.Sleep(2 * time.Second)
	if executed == 0 {
		t.Error("expected task to execute at least once")
	}
}

func TestRemoveTask(t *testing.T) {
	m := NewManager()
	ctx := context.Background()
	m.Start(ctx)
	defer m.Stop()
	
	m.AddCronTask("test", "@every 1s", func() {})
	if !m.RemoveTask("test") {
		t.Error("RemoveTask should return true")
	}
	if m.RemoveTask("test") {
		t.Error("RemoveTask should return false for missing task")
	}
}
