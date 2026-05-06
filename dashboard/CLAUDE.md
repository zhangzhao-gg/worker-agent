# dashboard/ - Node.js 独立 Dashboard
> L2 | 父级: /CLAUDE.md

Vite + React + Express + better-sqlite3，直连 `data/*.db` 读写，不依赖 Worker 进程

## 架构

```
server/index.js  — Express API，直连 SQLite 读写，安全码中间件保护写操作
src/main.jsx     — React 入口
src/App.jsx      — 路由（hash-free pushState）
src/index.css    — 全局样式：CSS 变量、墙面肌理、水渍色斑、暗角、字体定义
src/pages/       — WorkerList（列表）+ WorkerDetail（访客视图 + 详情六标签）
index.html       — SVG filter 定义（paper-edge 撕裂边缘）+ vignette 暗角元素
public/fonts/    — Special Elite 打字机字体 TTF
public/avatars/  — 工人头像静态文件
```

## 双视图架构

- **访客视图（默认）**：一屏状态卡，显示身份/当前活动/情绪叙事/最近日记+见闻/下次思考/唤醒输入/思考链实时展示
- **详情视图**：六标签全量数据（日记/内心OS/见闻/日程/思考计划/思维链），任何人可通过"了解更多"按钮进入
- **Admin 权限**：仅控制编辑人设和重置操作，隐蔽锁图标入口

## 视觉风格

黄皮纸 + 油墨 + 撕裂边缘 + 墙面做旧：
- 纸带底色 `#d4c4a8`，SVG feTurbulence displacement 产生不规则边缘
- 油墨 text-shadow 模拟墨水渗透（推理日志除外）
- 墙面背景 `#f0ebe4` + 颗粒肌理 + 水渍色斑 + 电影暗角
- 英文打字机字体 Special Elite，中文系统字体
- 思考面板"正在思考"逐字波浪动画
- 详见 `docs/ink-effect.md`

## 启动

```bash
cd dashboard && npm run dev
# 前端 :4000 + API :3001，Vite 自动代理 /api → :3001
```

## 安全码机制

写操作（编辑人设、重置）需要安全码验证：
- 后端：`ADMIN_CODE` 环境变量，`requireAdmin` 中间件校验 `x-admin-code` header
- 前端：详情视图侧边栏管理员验证，验证通过后显示编辑/重置按钮
- 验证接口：`POST /api/auth/verify`

## 部署

详见 `docs/deploy.md`（本地文件，未入 git）

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
