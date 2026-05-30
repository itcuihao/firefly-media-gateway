import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// Parse backend proxy port from environment variables
const listenAddr = process.env.APP_LISTEN_ADDR || ':8080'
const backendPort = listenAddr.includes(':') ? listenAddr.split(':').pop() : '8080'
const proxyTarget = process.env.VITE_PROXY_TARGET || `http://localhost:${backendPort}`

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
        target: proxyTarget,
        changeOrigin: true,
      }
    }
  }
})

