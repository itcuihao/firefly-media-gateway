package httpapi

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"firefly-media-gateway/uiembed"
)

// frontendHandler 返回一个 HTTP handler，将 /admin/ 路径下的请求映射到
// 嵌入的 Vue 3 + Naive UI 前端构建产物（frontend/dist）。
// 对于 SPA History 路由，回退到 index.html（带 no-cache 防止 CDN 缓存过期入口）；
// 对于丢失的静态资源（带有文件扩展名或位于 assets/ 下），直接返回 404，防止回退 HTML 导致模块加载 MIME 错误。
func (s *Server) frontendHandler() http.Handler {
	distFS, err := fs.Sub(uiembed.Dist, "dist")
	if err != nil {
		panic("embed frontend/dist failed: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path
		if reqPath == "" {
			reqPath = "/"
		}

		cleanPath := strings.TrimPrefix(reqPath, "/")

		// 1. 请求根路径或 index.html：直接禁用缓存，确保客户端/CDN每次获取最新入口
		if cleanPath == "" || cleanPath == "index.html" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			fileServer.ServeHTTP(w, r)
			return
		}

		// 2. 检查静态文件是否存在
		f, openErr := distFS.Open(cleanPath)
		if openErr == nil {
			_ = f.Close()
			// 存在该静态资源
			if strings.HasPrefix(cleanPath, "assets/") {
				// Vite 构建的带 hash 静态资源，可安全强缓存 1 年
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// 3. 文件不存在：判断是静态文件还是 SPA 页面路由
		// 如果在 assets/ 目录下，或者文件名包含扩展名（如 .js, .css, .map, .png），说明是丢失的静态资源，必须返回 404
		base := path.Base(cleanPath)
		if strings.HasPrefix(cleanPath, "assets/") || strings.Contains(base, ".") {
			http.NotFound(w, r)
			return
		}

		// 4. SPA 页面路由（如 /dashboard, /gallery, /sandbox），回退到 index.html 并禁用缓存
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
