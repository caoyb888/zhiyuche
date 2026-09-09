import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// 后端 API 缺省监听 20080；前端 dev server 缺省 20173（共享机器仅允许 20000-20999）。
// 代理目标可通过环境变量 VITE_DEV_PROXY_TARGET（进程环境或 .env 文件）覆盖；
// 端口可通过 `vite --port <n>` 覆盖，strictPort 保证端口被占用时直接失败而不是静默换端口。
const DEFAULT_API_TARGET = 'http://localhost:20080'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'VITE_')
  const apiTarget = process.env.VITE_DEV_PROXY_TARGET ?? env.VITE_DEV_PROXY_TARGET ?? DEFAULT_API_TARGET

  return {
    plugins: [react()],
    server: {
      host: true,
      port: 20173,
      strictPort: true,
      proxy: {
        '/api': {
          target: apiTarget,
          changeOrigin: true,
        },
      },
    },
    preview: {
      host: true,
      port: 20174,
      strictPort: true,
    },
  }
})
