import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    tailwindcss()
  ],
  base: '/debug/ui/',
  build: {
    // 构建产物直接输出到 uiembed/dist，Go embed 可以直接嵌入
    outDir: '../uiembed/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      }
    }
  }
})

