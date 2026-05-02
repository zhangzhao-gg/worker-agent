# 部署手册

## 当前部署状态

| 项目 | 值 |
|------|-----|
| 服务器 | 81.70.158.243 (Ubuntu 24.04) |
| SSH | `ssh ubuntu@81.70.158.243` |
| 域名 | https://worker.okethan.top |
| HTTPS | Let's Encrypt，自动续期，到期 2026-07-31 |
| 代码路径 | `/opt/worker-agent/` |
| 数据路径 | `/opt/worker-agent/data/` |
| Worker 端口 | 8080（仅内网） |
| Dashboard API 端口 | 3001（Nginx 反代） |

环境已装好：Go 1.22、Node 20、Nginx 1.24、certbot。**后续更新不需要重装环境。**

## 日常更新（最常用）

本机执行：

```bash
# 1. 推送代码
rsync -avz --exclude='node_modules' --exclude='.git' --exclude='dashboard/dist' --exclude='data' \
  -e ssh /Users/zhangzhao/Desktop/worspace/worker-agent/ ubuntu@81.70.158.243:/opt/worker-agent/

# 2. 重新编译 + 构建 + 重启
ssh ubuntu@81.70.158.243 "cd /opt/worker-agent && go build -o worker ./cmd/worker && cd dashboard && npm install && npx vite build && sudo systemctl restart worker-agent dashboard-api"
```

如果只改了前端（jsx/css）：

```bash
rsync -avz --exclude='node_modules' --exclude='.git' --exclude='dist' \
  -e ssh /Users/zhangzhao/Desktop/worspace/worker-agent/dashboard/ ubuntu@81.70.158.243:/opt/worker-agent/dashboard/

ssh ubuntu@81.70.158.243 "cd /opt/worker-agent/dashboard && npx vite build && sudo systemctl restart dashboard-api"
```

如果只改了 Go 后端：

```bash
rsync -avz --exclude='node_modules' --exclude='.git' --exclude='dashboard/dist' --exclude='data' \
  -e ssh /Users/zhangzhao/Desktop/worspace/worker-agent/ ubuntu@81.70.158.243:/opt/worker-agent/

ssh ubuntu@81.70.158.243 "cd /opt/worker-agent && go build -o worker ./cmd/worker && sudo systemctl restart worker-agent"
```

## 数据同步

```bash
# 本地 → 服务器
rsync -avz -e ssh /Users/zhangzhao/Desktop/worspace/worker-agent/data/ ubuntu@81.70.158.243:/opt/worker-agent/data/

# 服务器 → 本地（备份）
rsync -avz -e ssh ubuntu@81.70.158.243:/opt/worker-agent/data/ /Users/zhangzhao/Desktop/worspace/worker-agent/data/
```

## 运维命令

```bash
# 查看服务状态
ssh ubuntu@81.70.158.243 "sudo systemctl status worker-agent dashboard-api"

# 查看日志
ssh ubuntu@81.70.158.243 "sudo journalctl -u worker-agent -f"
ssh ubuntu@81.70.158.243 "sudo journalctl -u dashboard-api -f"

# 重启服务
ssh ubuntu@81.70.158.243 "sudo systemctl restart worker-agent"
ssh ubuntu@81.70.158.243 "sudo systemctl restart dashboard-api"

# 停止服务
ssh ubuntu@81.70.158.243 "sudo systemctl stop worker-agent dashboard-api"
```

## 架构图

```
浏览器 → Nginx (:443 HTTPS)
            ├── /api/*  → Express (:3001) → SQLite
            └── /*      → dist/ 静态文件

Worker 进程 (:8080) → SQLite（仅内网，不暴露）
```

---

## 首次部署参考（已完成，留档备查）

以下步骤已在 2026-05-03 执行完毕，后续更新无需重复。

### 环境安装

```bash
apt update && apt install -y nginx certbot python3-certbot-nginx git build-essential python3 golang-go
curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && apt install -y nodejs
```

### Systemd 服务文件

**`/etc/systemd/system/worker-agent.service`**

```ini
[Unit]
Description=Worker Agent
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/opt/worker-agent
Environment=LLM_API_KEY=<API Key>
ExecStart=/opt/worker-agent/worker -port 8080 -data /opt/worker-agent/data
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**`/etc/systemd/system/dashboard-api.service`**

```ini
[Unit]
Description=Worker Dashboard API
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/opt/worker-agent/dashboard
Environment=DATA_DIR=/opt/worker-agent/data
Environment=PORT=3001
ExecStart=/usr/bin/node server/index.js
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Nginx 配置

`/etc/nginx/sites-available/worker.okethan.top`（certbot 已自动添加 HTTPS 块）：

```nginx
server {
    listen 80;
    server_name worker.okethan.top;

    root /opt/worker-agent/dashboard/dist;
    index index.html;

    location /api/ {
        proxy_pass http://127.0.0.1:3001;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

### Worker CLI 参数

```
-city string    城市 API 地址（空则启用 mock 模式）
-data string    数据目录 (default "./data")
-llm-key string LLM API Key（默认读 LLM_API_KEY 环境变量）
-llm-model string LLM 模型名称 (default "MiniMax-M2.7")
-llm-url string LLM API 地址 (default "https://api.minimaxi.com")
-port int       HTTP 服务端口 (default 8080)
```
