# dashboard/ - Node.js 独立 Dashboard
> L2 | 父级: /CLAUDE.md

Vite + React + Express + better-sqlite3，直连 `data/*.db` 读写，不依赖 Worker 进程

## 架构

```
server/index.js  — Express API，直连 SQLite 读写，安全码中间件保护写操作
src/main.jsx     — React 入口
src/App.jsx      — 路由（hash-free pushState）
src/pages/       — WorkerList（列表）+ WorkerDetail（详情六标签 + 翻页 + 安全码验证）
```

## 启动

```bash
cd dashboard && npm run dev
# 前端 :4000 + API :3001，Vite 自动代理 /api → :3001
# nodemon 监听后端文件变更自动重启
```

## 安全码机制

写操作（编辑人设、重置）需要安全码验证：
- 后端：`ADMIN_CODE` 环境变量，`requireAdmin` 中间件校验 `x-admin-code` header
- 前端：侧边栏 ADMIN ACCESS 输入框，验证通过后显示 EDIT/RESET 按钮
- 验证接口：`POST /api/auth/verify`

## Reasoning Tab 设计

按 session_id 分组，左侧 session 列表 + 右侧详情，支持 COPY ALL

## Schedule/Wakeup Tab

客户端分页（心跳每页 30 条，唤醒每页 20 条），PREV/NEXT 翻页

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
