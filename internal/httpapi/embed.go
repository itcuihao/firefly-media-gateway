package httpapi

import (
	"io/fs"
	"net/http"
	"strings"

	"firefly-media-gateway/uiembed"
)

// frontendHandler 返回一个 HTTP handler，将 /admin/ 路径下的请求映射到
// 嵌入的 Vue 3 + Naive UI 前端构建产物（frontend/dist）。
// 对于不存在的资源路径，一律回退到 index.html（SPA History 路由兼容）。
func (s *Server) frontendHandler() http.Handler {
	distFS, err := fs.Sub(uiembed.Dist, "dist")
	if err != nil {
		panic("embed frontend/dist failed: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 已经被 StripPrefix 处理，r.URL.Path 为 /assets/xxx 或 /
		path := r.URL.Path
		if path == "" {
			path = "/"
		}

		// 尝试是否存在该静态文件，不存在则 fallback 到 index.html（SPA路由）
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath != "" {
			if _, err := distFS.Open(cleanPath); err != nil {
				// 文件不存在 → 回退到 SPA 入口
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				fileServer.ServeHTTP(w, r2)
				return
			}
		}

		fileServer.ServeHTTP(w, r)
	})
}
