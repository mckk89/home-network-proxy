package webdav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewServer(t *testing.T) {
	root := t.TempDir()
	s := NewServer(root, false)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.root != root {
		t.Errorf("expected root=%s, got %s", root, s.root)
	}
}

func TestStartStop(t *testing.T) {
	root := t.TempDir()
	s := NewServer(root, false)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// 使用随机端口
	if err := s.Start(ctx, 18099); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	
	if !s.running {
		t.Error("expected running=true")
	}
	
	s.Stop()
	if s.running {
		t.Error("expected running=false")
	}
}

func TestHandlePropfind(t *testing.T) {
	root := t.TempDir()
	s := NewServer(root, false)
	
	// 创建测试目录和文件
	testDir := filepath.Join(root, "test")
	os.MkdirAll(testDir, 0755)
	os.WriteFile(filepath.Join(testDir, "file.txt"), []byte("test content"), 0644)
	
	// 模拟 PROPFIND 请求
	req := httptest.NewRequest("PROPFIND", "/test/", nil)
	rec := httptest.NewRecorder()
	
	s.handleDAV(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	
	body := rec.Body.String()
	if !contains(body, "test") {
		t.Error("response should contain directory listing")
	}
}

func TestHandleGet(t *testing.T) {
	root := t.TempDir()
	s := NewServer(root, false)
	
	// 创建测试文件
	testFile := filepath.Join(root, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)
	
	// 模拟 GET 请求
	req := httptest.NewRequest("GET", "/test.txt", nil)
	rec := httptest.NewRecorder()
	
	s.handleDAV(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	
	body := rec.Body.String()
	if body != "hello world" {
		t.Errorf("expected 'hello world', got %q", body)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
