package web

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// LoadTemplates 加载内嵌的 HTML 模板。
func LoadTemplates() (*template.Template, error) {
	return template.ParseFS(templatesFS, "templates/*.html")
}

// StaticFS 返回内嵌的静态资源文件系统。
func StaticFS() (fs.FS, error) {
	return fs.Sub(staticFS, "static")
}
