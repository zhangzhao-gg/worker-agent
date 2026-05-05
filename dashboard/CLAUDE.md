# dashboard/ - Node.js 独立 Dashboard
> L2 | 父级: /CLAUDE.md

Vite + React + Express + better-sqlite3，直连 `data/*.db` 读写，不依赖 Worker 进程

## 架构

```
server/index.js  — Express API，直连 SQLite 读写，安全码中间件保护写操作
src/main.jsx     — React 入口
src/App.jsx      — 路由（hash-free pushState）
src/index.css    — 全局样式：CSS 变量、墙面肌理、水渍色斑、暗角、字体定义
src/pages/       — WorkerList（列表）+ WorkerDetail（详情六标签 + 翻页 + 安全码验证）
index.html       — SVG filter 定义（paper-edge 撕裂边缘）+ vignette 暗角元素
public/fonts/    — Special Elite 打字机字体 TTF
public/avatars/  — 工人头像静态文件
```

## 视觉风格

黄皮纸 + 油墨 + 撕裂边缘 + 墙面做旧：
- 纸带底色 `#d4c4a8`，SVG feTurbulence displacement 产生不规则边缘
- 油墨 text-shadow 模拟墨水渗透（推理日志除外）
- 墙面背景 `#f0ebe4` + 颗粒肌理 + 水渍色斑 + 电影暗角
- 英文打字机字体 Special Elite，中文系统字体
- 详见 `docs/ink-effect.md`

## 启动

```bash
cd dashboard && npm run dev
# 前端 :4000 + API :3001，Vite 自动代理 /api → :3001
```

## 安全码机制

写操作（编辑人设、重置）需要安全码验证：
- 后端：`ADMIN_CODE` 环境变量，`requireAdmin` 中间件校验 `x-admin-code` header
- 前端：侧边栏管理员验证输入框，验证通过后显示编辑/重置按钮
- 验证接口：`POST /api/auth/verify`

## 部署

```bash
rsync -avz --exclude='node_modules' --exclude='.git' --exclude='dist' \
  -e ssh dashboard/ ubuntu@81.70.158.243:/opt/worker-agent/dashboard/
ssh ubuntu@81.70.158.243 "cd /opt/worker-agent/dashboard && npx vite build && sudo systemctl restart dashboard-api"
```

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
