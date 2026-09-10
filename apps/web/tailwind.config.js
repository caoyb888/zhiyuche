/**
 * 方向 A「深空控制台」暗色令牌。
 *
 * 语义分层：
 *   surface-*  承重表面，0 最暗（页面底），数字越大抬得越高（卡片 → 表头 → 选中）
 *   line-*     描边，暗色下层次主要靠描边而非阴影
 *   ink-*      文字墨阶，DEFAULT 为正文
 *   brand-*    行动蓝：600/700 用作填充，300/400 才可作暗底上的文字与链接
 *   tech-*     电光青，只用于「活的」数据：实时推送、遥测、轨迹
 * 语义色（ev/warn/danger/violet）在暗底上的用法统一为
 *   填充 color/10~15、描边 color/30、文字取 200 档。
 */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        surface: {
          0: '#050E1C', // 页面底
          1: '#081426', // 侧栏 / 顶栏
          2: '#0D1E33', // 卡片 / 面板
          3: '#132844', // 表头 / hover / 内嵌块
          4: '#1A3555', // 选中 / 输入框
        },
        line: {
          DEFAULT: '#1A3350',
          soft: '#12253C',
          strong: '#23456B',
        },
        ink: {
          DEFAULT: '#DCE7F2',
          strong: '#F2F7FC',
          muted: '#93AAC4',
          faint: '#7089A8',
          disabled: '#465E7E',
        },
        brand: {
          50: '#EEF5FD', 100: '#DBEAFE', 200: '#BFDBFE',
          300: '#8FBDF7', 400: '#5C9DF0', 500: '#2B7FE6',
          600: '#1D6FD8', 700: '#1657B0', 800: '#134276', 900: '#0F2A4D',
        },
        tech:   { 200: '#9FE4EF', 400: '#3CCFE0', 500: '#12B0C6', 700: '#0E7F92' },
        ev:     { 200: '#6EE7B7', 400: '#34D399', 500: '#10B981', 600: '#059669' },
        warn:   { 200: '#F5C168', 400: '#F0A32B', 500: '#E08C0B' },
        danger: { 200: '#FFA79E', 400: '#EF4B3C', 500: '#E0392C' },
        violet: { 200: '#C4B5FD', 400: '#9B7CF6', 500: '#7C5CF0' },
      },
      // 暗底叠色配方用到的档位（12/15 填充、35/45 描边、85 浮层底）不在 Tailwind
      // 默认 opacity 表里，不显式登记的话 bg-ev-500/15 这类类名会被静默丢弃。
      opacity: {
        12: '0.12',
        15: '0.15',
        35: '0.35',
        45: '0.45',
        85: '0.85',
      },
      fontFamily: {
        sans: ['"IBM Plex Sans SC"', '"IBM Plex Sans"', '"Source Han Sans SC"', '"PingFang SC"', '"Microsoft YaHei"', 'system-ui', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'SFMono-Regular', 'ui-monospace', 'Consolas', 'monospace'],
      },
      boxShadow: {
        panel: '0 1px 0 0 rgba(255,255,255,.03) inset, 0 8px 24px -16px rgba(0,0,0,.85)',
        float: '0 24px 60px -24px rgba(0,0,0,.9)',
        glow: '0 0 10px rgba(60,207,224,.85)',
      },
      borderRadius: {
        card: '10px',
      },
    },
  },
  plugins: [],
}
