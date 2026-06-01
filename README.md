# 智驭车 (ZhiYuChe)

智驭车是一款基于 React + TypeScript + Vite 构建的智能车辆管理平台前端 Demo，提供仪表盘、行程管理、充电监控、车辆健康、审批流程及数据报表等核心模块。

## 技术栈

- **框架**: React 18 + TypeScript
- **构建工具**: Vite 5
- **样式**: Tailwind CSS
- **图表**: Recharts
- **路由**: React Router DOM 6
- **图标**: Lucide React

## 一键部署

点击下方按钮即可将本项目一键部署到 Vercel：

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https%3A%2F%2Fgithub.com%2Fcaoyb888%2Fzhiyuche)

## 本地开发

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 构建生产版本
npm run build

# 预览生产构建
npm run preview
```

## Vercel 手动部署

1. 访问 [Vercel Dashboard](https://vercel.com/new)
2. 导入 GitHub 仓库 `caoyb888/zhiyuche`
3. 框架预设选择 **Vite**
4. 点击 **Deploy**

Vercel 会自动识别构建命令（`npm run build`）和输出目录（`dist`），并配置好 SPA 路由回退。

## 项目结构

```
src/
├── pages/          # 页面组件（Dashboard, Trips, Charging, Health, Approval, Reports）
├── components/     # 公共组件（Layout 等）
├── data/           # 模拟数据
├── assets/         # 静态资源
├── App.tsx         # 根组件
└── main.tsx        # 入口文件
```

## License

MIT
