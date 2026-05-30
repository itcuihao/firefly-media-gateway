import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig(() => {
  // Parse port from environment variable if present, defaulting to 8088
  const listenAddr = process.env.APP_LISTEN_ADDR || ''
  const envPort = listenAddr.includes(':') ? listenAddr.split(':').pop() : '8088'
  const activePort = envPort || '8088'

  const proxyTarget = process.env.VITE_PROXY_TARGET || `http://127.0.0.1:${activePort}`

  console.log(`[Vite Proxy] Routing API requests to: ${proxyTarget}`)

  return {
    plugins: [
      vue(),
      tailwindcss()
    ],
    base: '/admin/',
    build: {
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
  }
})


