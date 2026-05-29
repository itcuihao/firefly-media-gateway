// Package uiembed 将 Vue 3 + Naive UI 构建产物嵌入到最终 Go 二进制中。
// Vite 构建时通过 outDir: '../uiembed/dist' 将产物输出到本目录下的 dist/。
package uiembed

import "embed"

// Dist 嵌入 Vite 构建输出的所有静态文件（由 frontend/ 项目生成到本目录 dist/）。
//
//go:embed all:dist
var Dist embed.FS
