import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import http from 'http'

// Helper to check if a backend port is listening and responsive
function checkPort(port: number): Promise<boolean> {
  return new Promise((resolve) => {
    const req = http.request(
      {
        hostname: '127.0.0.1',
        port: port,
        path: '/api/v1/health',
        method: 'GET',
        timeout: 200,
      },
      (res) => {
        resolve(res.statusCode === 200)
      }
    )
    req.on('error', () => resolve(false))
    req.on('timeout', () => {
      req.destroy()
      resolve(false)
    })
    req.end()
  })
}

// https://vite.dev/config/
export default defineConfig(async () => {
  // Parse port from environment variable if present
  const listenAddr = process.env.APP_LISTEN_ADDR || ''
  const envPort = listenAddr.includes(':') ? parseInt(listenAddr.split(':').pop() || '') : null

  let activePort = 8080

  if (envPort && await checkPort(envPort)) {
    activePort = envPort
  } else if (await checkPort(8088)) {
    activePort = 8088
  } else if (await checkPort(8080)) {
    activePort = 8080
  } else {
    // If no running server is detected, fall back to the env port or 8080 default
    activePort = envPort || 8080
  }

  const proxyTarget = process.env.VITE_PROXY_TARGET || `http://127.0.0.1:${activePort}`

  console.log(`[Vite Proxy] Routing API requests to: ${proxyTarget}`)

  return {
    plugins: [
      vue(),
      tailwindcss()
    ],
    base: '/debug/ui/',
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

