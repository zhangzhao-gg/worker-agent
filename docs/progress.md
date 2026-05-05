# Worker Agent 开发进度

## 项目状态：线上运行中

系统已完成完整链路并部署上线：创建工人 → LLM 思考 → 感知城市 → 制定计划 → 写叙事/记忆 → 更新情绪 → 安排下次唤醒。

线上地址: https://worker.okethan.top

---

## 已完成

### 核心架构
- **单进程多工人**：HTTP API + 协程管理所有工人生命周期
- **双协程模型**：心跳协程（身体）+ 唤醒调度协程（大脑入口）
- **SQLite WAL 模式**：7 张表（含 reasoning_logs），支持并发读写
- **LLM Agent Loop**：最多 30 轮推理，含 microcompact + autoCompact 压缩管线
- **16 个工具**：6 感知 + 8 行动（含 cancel_wakeup / self_destruct）+ 2 元工具
- **TodoManager**：防止 LLM 推理偏移，3 轮未更新自动提醒

### 唤醒/心跳制度
- 工作制度：朝八晚六（08:00-18:00），每 10 分钟心跳汇报
- 唤醒规则：一天 2-3 次，LLM 自主安排
- cancel_wakeup：LLM 可自主清理冗余唤醒
- self_destruct：LLM 在极端绝望时可选择自我终结
- 重启审视：每次重启无条件触发一次 LLM 推理

### Dashboard
- **视觉风格**：黄皮纸纸带 + SVG displacement 撕裂边缘 + 油墨文字效果 + 墙面做旧背景 + 电影暗角
- **字体**：英文 Special Elite 打字机字体（本地 TTF），中文系统字体
- **六标签详情页**：对外叙事、私人记忆（模糊保护）、城市事件、心跳计划、唤醒计划、推理日志
- **推理日志**：按 session 分组，左列表右详情，支持复制
- **安全码保护**：编辑人设、重置操作需验证安全码
- **头像支持**：工人可配置头像
- **交互**：唤醒成功后自动跳转唤醒计划 tab
- **UI 全中文化**

### 健壮性
- 唤醒失败重试：LLM 调用失败时 wakeup 保留 pending
- 防消失兜底：LLM 忘记安排 wakeup 时自动补插次日早晨唤醒
- 推理锁 + 审视机制
- 重启自动恢复：扫描 data/*.db，恢复所有工人

### 部署
- HTTPS：Let's Encrypt 自动续期
- 进程管理：systemd（worker-agent + dashboard-api）
- 反代：Nginx，静态文件 + API 代理
- 详细配置见 docs/deploy.md（本地文件，未入 git）

---

## 待完成

### 功能
- [ ] 城市 API 真实 HTTP 对接（目前仅 mock）

### 优化
- [ ] 多工人并发时的 LLM 调用限流
- [ ] 压缩管线实战验证（长对话场景）

---

*最后更新：2026-05-05*
