package webdav

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Server WebDAV 服务器
type Server struct {
	root     string
	readonly bool
	port     int
	server   *http.Server
	mu       sync.RWMutex
	running  bool
}

// NewServer 创建 WebDAV 服务器。
func NewServer(root string, readonly bool) *Server {
	return &Server{
		root:     root,
		readonly: readonly,
	}
}

// Start 启动 WebDAV 服务（非阻塞，后台运行）。
func (s *Server) Start(ctx context.Context, port int) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	// 确保根目录存在
	if err := os.MkdirAll(s.root, 0755); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("创建 WebDAV 根目录失败: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/dav", s.handleDAVRoot)
	mux.HandleFunc("/", s.handleDAV)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	s.running = true
	s.port = port
	s.mu.Unlock()

	// 后台运行
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("WebDAV 服务器错误: %v", err)
		}
	}()

	return nil
}

// Stop 停止服务。
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	server := s.server
	s.mu.Unlock()

	if server != nil {
		return server.Close()
	}
	return nil
}

// handleDAVRoot 返回 DAV 协议发现响应。
func (s *Server) handleDAVRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("DAV", "1")
	w.Header().Set("Allow", "OPTIONS, GET, HEAD, PROPFIND")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<xml xmlns=\"DAV:\"><prop><resourcetype/></prop></xml>"))
}

// handleDAV 处理 WebDAV 请求。
func (s *Server) handleDAV(w http.ResponseWriter, r *http.Request) {
	// 解析路径
	relPath := strings.TrimPrefix(r.URL.Path, "/")
	if relPath == "" {
		relPath = "."
	}

	// 安全检查：防止目录遍历
	absPath := filepath.Join(s.root, filepath.Clean(relPath))
	if !strings.HasPrefix(absPath, filepath.Clean(s.root)) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodOptions:
		s.handleOptions(w, r)
	case "PROPFIND":
		s.handlePropfind(w, r, absPath)
	case http.MethodGet:
		s.handleGet(w, r, absPath)
	case "MKCOL":
		s.handleMkcol(w, r, absPath)
	case http.MethodPut:
		if s.readonly {
			http.Error(w, "Read-only", http.StatusForbidden)
			return
		}
		s.handlePut(w, r, absPath)
	case http.MethodDelete:
		if s.readonly {
			http.Error(w, "Read-only", http.StatusForbidden)
			return
		}
		s.handleDelete(w, r, absPath)
	case "COPY":
		s.handleCopy(w, r, absPath)
	case "MOVE":
		s.handleMove(w, r, absPath)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// handleOptions 返回允许的方法。
func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("DAV", "1")
	w.Header().Set("Allow", "OPTIONS, GET, HEAD, PROPFIND, MKCOL, PUT, DELETE, COPY, MOVE")
	w.Header().Set("MS-Author-Via", "DSP")
	w.WriteHeader(http.StatusOK)
}

// handlePropfind 返回目录或文件属性。
func (s *Server) handlePropfind(w http.ResponseWriter, r *http.Request, absPath string) {
	info, err := os.Stat(absPath)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// 构建 PROPFIND 响应
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">`)

	if info.IsDir() {
		// 返回目录自身
		builder.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>%s/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>%s</D:displayname>
        <D:resourcetype><D:collection/></D:resourcetype>
        <D:contentlength>0</D:contentlength>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, r.URL.Path, filepath.Base(absPath)))

		// 列出子项
		entries, _ := os.ReadDir(absPath)
		for _, entry := range entries {
			subPath := filepath.Join(absPath, entry.Name())
			subInfo, _ := os.Stat(subPath)
			if subInfo == nil {
				continue
			}
			builder.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>%s/%s</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>%s</D:displayname>
        <D:resourcetype>%s</D:resourcetype>
        <D:contentlength>%d</D:contentlength>
        <D:getlastmodified>%s</D:getlastmodified>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`,
				r.URL.Path,
				entry.Name(),
				entry.Name(),
				resourceType(entry),
				subInfo.Size(),
				subInfo.ModTime().Format(http.TimeFormat),
			))
		}
	} else {
		// 返回文件
		builder.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>%s</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>%s</D:displayname>
        <D:resourcetype/>
        <D:contentlength>%d</D:contentlength>
        <D:getlastmodified>%s</D:getlastmodified>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`,
			r.URL.Path,
			filepath.Base(absPath),
			info.Size(),
			info.ModTime().Format(http.TimeFormat),
		))
	}

	builder.WriteString(`
</D:multistatus>`)
	w.Write([]byte(builder.String()))
}

// resourceType 返回 DAV 资源类型标签。
func resourceType(entry os.DirEntry) string {
	if entry.IsDir() {
		return `<D:collection/>`
	}
	return ``
}

// handleGet 返回文件内容。
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, absPath string) {
	file, err := os.Open(absPath)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("Last-Modified", info.ModTime().Format(http.TimeFormat))
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// handleMkcol 创建目录。
func (s *Server) handleMkcol(w http.ResponseWriter, r *http.Request, absPath string) {
	if s.readonly {
		http.Error(w, "Read-only", http.StatusForbidden)
		return
	}
	if err := os.MkdirAll(absPath, 0755); err != nil {
		http.Error(w, "Created", http.StatusCreated)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// handlePut 写入文件。
func (s *Server) handlePut(w http.ResponseWriter, r *http.Request, absPath string) {
	if s.readonly {
		http.Error(w, "Read-only", http.StatusForbidden)
		return
	}

	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 100<<20))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := os.WriteFile(absPath, body, 0644); err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// handleDelete 删除文件或目录。
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, absPath string) {
	if s.readonly {
		http.Error(w, "Read-only", http.StatusForbidden)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	var removeErr error
	if info.IsDir() {
		removeErr = os.RemoveAll(absPath)
	} else {
		removeErr = os.Remove(absPath)
	}
	if removeErr != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleCopy 复制文件。
func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request, absPath string) {
	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Destination Required", http.StatusBadRequest)
		return
	}
	// 简化实现：仅处理同目录内复制
	log.Printf("COPY %s -> %s (未实现)", absPath, dest)
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}

// handleMove 移动文件。
func (s *Server) handleMove(w http.ResponseWriter, r *http.Request, absPath string) {
	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Destination Required", http.StatusBadRequest)
		return
	}
	log.Printf("MOVE %s -> %s (未实现)", absPath, dest)
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}

// Status 返回服务器状态。
func (s *Server) Status() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{
		"running":  s.running,
		"root":     s.root,
		"readonly": s.readonly,
		"port":     s.port,
	}
}

// 防止 xml 包未使用
var _ = xml.NewEncoder
var _ = time.Now
