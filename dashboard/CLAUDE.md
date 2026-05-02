# dashboard/ - Node.js 独立 Dashboard
> L2 | 父级: /CLAUDE.md

Vite + React + Express + better-sqlite3，纯只读直连 `data/*.db`，不侵入 Worker Agent

## 架构

```
server/index.js  — Express API，直读 SQLite，写操作代理到 Worker API
src/main.jsx     — React 入口
src/App.jsx      — 路由（hash-free pushState）
src/pages/       — WorkerList（列表）+ WorkerDetail（详情六标签）
```

## 启动

```bash
cd dashboard && npm run dev
# 前端 :5173 + API :3001，Vite 自动代理 /api → :3001
```

## Reasoning Tab 设计

增量轮询（after=maxId），DOM 不重建，滚动位置由 React ref 控制，支持 auto-scroll 开关

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
