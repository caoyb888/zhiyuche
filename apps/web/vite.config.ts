import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 后端 API 固定监听 20080；前端 dev server 使用 20173（共享机器仅允许 20000-20999）
const API_TARGET = 'http://localhost:20080'

export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    port: 20173,
    strictPort: true,
    proxy: {
      '/api': {
        target: API_TARGET,
        changeOrigin: true,
      },
    },
  },
  preview: {
    host: true,
    port: 20174,
    strictPort: true,
  },
})
