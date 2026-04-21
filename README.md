# cliproxy-feishu-monitor

一个面向服务器部署的 CLIProxyAPI 监控服务。

功能目标：
- 读取 CLIProxyAPI 管理接口中的 Codex 账号
- 查询每个账号的 `code-5h` / `code-7d` 配额状态
- 汇总账号状态：充足 / 高 / 中 / 低 / 耗尽 / 禁用 / 异常
- 汇总 Token 用量：7 小时 / 24 小时 / 7 天 / 累计
- 推送到飞书机器人
- 支持 `run-once` 单次执行
- 支持 `serve` 常驻运行
- 支持启动通知、3 小时心跳、6 小时正式汇总
- 支持 systemd 安装和 Linux 离线发布包

---

## 当前默认运行策略

- 正式汇总推送：**每 6 小时 1 次**
- 健康心跳推送：**每 3 小时 1 次**
- 启动通知：**开启**
- 服务启动后立即执行一次汇总：**开启**

---

## 消息格式

飞书消息默认使用中文，Token 用量显示为 **原始总量**（千分位格式）。

示例：

```text
状态概览
来源: http://YOUR_CPA_HOST:8317
时间: 2026-04-21 19:52:15 CST

账号总数 166 | 充足 14 | 高 94 | 中 25 | 低 12 | 耗尽 9 | 禁用 5 | 异常 7

汇总
7日免费等效: 11304%
7小时 Token 用量: 38,130,000
24小时 Token 用量: 109,250,000
7天 Token 用量: 239,450,000
累计 Token 用量: 309,830,000
```

---

## 配置文件

推荐使用本地 JSON 配置文件：

- `local.runtime.json`
- `runtime.local.json`
- `config.local.json`

优先读取本地 JSON；如果这些文件不存在，才回退环境变量。

先复制示例：

```bash
cp local.runtime.json.example local.runtime.json
```

示例内容：

```json
{
  "cpa_base_url": "http://YOUR_CPA_HOST:8317/management.html#/login",
  "management_key": "replace-me",
  "feishu_webhook": "https://open.feishu.cn/open-apis/bot/v2/hook/replace-me",
  "feishu_secret": "replace-me",
  "poll_interval_hours": 6,
  "heartbeat_interval_hours": 3,
  "heartbeat_enabled": true,
  "startup_notification_enabled": true,
  "run_summary_on_startup": true,
  "request_timeout_seconds": 30,
  "concurrency": 8,
  "failure_alert_threshold": 3,
  "state_path": "data/runtime-state.json"
}
```

字段说明：
- `cpa_base_url`：CLIProxyAPI 地址，可直接填管理页地址
- `management_key`：管理密钥
- `feishu_webhook`：飞书机器人 webhook
- `feishu_secret`：飞书签名密钥
- `poll_interval_hours`：正式汇总推送间隔（小时）
- `heartbeat_interval_hours`：健康心跳间隔（小时）
- `heartbeat_enabled`：是否启用心跳
- `startup_notification_enabled`：是否推送启动通知
- `run_summary_on_startup`：服务启动后是否立即推送一次汇总
- `request_timeout_seconds`：请求超时秒数
- `concurrency`：并发查询账号数
- `failure_alert_threshold`：连续失败多少次后发异常告警
- `state_path`：运行状态文件路径

---

## 本地运行

### 单次执行，不推送

```bash
go run . run-once --dry-run
```

### 单次执行，真实推送

```bash
go run . run-once
```

### 常驻运行

```bash
go run . serve
```

---

## 本地构建

```bash
go build -o dist/cliproxy-feishu-monitor .
```

---

## Linux 离线发布包

构建 Linux x86_64 发布包：

```bash
bash deploy/build-release.sh
```

产物：

```text
dist/cliproxy-feishu-monitor-linux-x86_64.tar.gz
```

解压后包含：
- `cliproxy-feishu-monitor`
- `local.runtime.json.example`
- `install-service.sh`
- `run-once.sh`
- `service-status.sh`
- `service-logs.sh`

---

## 最短部署命令

如果你只想要一条最短可用命令，在服务器上直接执行下面这段即可：

```bash
curl -fsSL https://raw.githubusercontent.com/changdaye/cliproxy-feishu-monitor/main/deploy/deploy-from-tar.sh -o deploy-from-tar.sh && chmod +x deploy-from-tar.sh && \
CPA_BASE_URL="http://YOUR_CPA_HOST:8317/management.html#/login" \
CPA_MANAGEMENT_KEY="你的管理密钥" \
FEISHU_WEBHOOK="你的飞书 webhook" \
FEISHU_SECRET="你的飞书签名密钥" \
bash deploy-from-tar.sh --tar-url "https://github.com/changdaye/cliproxy-feishu-monitor/releases/download/v0.1.2/cliproxy-feishu-monitor-linux-x86_64.tar.gz"
```

执行完成后查看服务：

```bash
systemctl status cliproxy-feishu-monitor
journalctl -u cliproxy-feishu-monitor -f
```

---

## 服务器快速部署（推荐）

下面以 Linux x86_64 服务器为例。

### 1. 本地准备发布包

```bash
cd /Users/changdaye/Documents/cliproxy-feishu-monitor
bash deploy/build-release.sh
```

生成文件：

```text
dist/cliproxy-feishu-monitor-linux-x86_64.tar.gz
```

### 2. 直接一键部署（从 tar 包 URL 开始）

如果你已经把 tar 包放到一个可下载 URL，可以在服务器上直接运行：

```bash
curl -fsSL https://raw.githubusercontent.com/REPLACE_ME/cliproxy-feishu-monitor/main/deploy/deploy-from-tar.sh -o deploy-from-tar.sh
chmod +x deploy-from-tar.sh

CPA_BASE_URL="http://YOUR_CPA_HOST:8317/management.html#/login" \
CPA_MANAGEMENT_KEY="replace-me" \
FEISHU_WEBHOOK="https://open.feishu.cn/open-apis/bot/v2/hook/replace-me" \
FEISHU_SECRET="replace-me" \
bash deploy-from-tar.sh \
  --tar-url "https://YOUR_DOWNLOAD_HOST/cliproxy-feishu-monitor-linux-x86_64.tar.gz"
```

这个脚本会自动：
- 下载 tar.gz
- 解压发布包
- 生成 `local.runtime.json`（如果你没额外提供配置文件）
- 调用 `install-service.sh`
- 启动 systemd 服务

如果你已经有现成配置文件，也可以这样：

```bash
bash deploy-from-tar.sh \
  --tar-url "https://YOUR_DOWNLOAD_HOST/cliproxy-feishu-monitor-linux-x86_64.tar.gz" \
  --config-file /root/local.runtime.json
```

或者从 URL 拉配置：

```bash
bash deploy-from-tar.sh \
  --tar-url "https://YOUR_DOWNLOAD_HOST/cliproxy-feishu-monitor-linux-x86_64.tar.gz" \
  --config-url "https://YOUR_DOWNLOAD_HOST/local.runtime.json"
```

### 3. 上传到服务器

```bash
scp dist/cliproxy-feishu-monitor-linux-x86_64.tar.gz root@YOUR_SERVER_IP:/root/
```

### 3. 在服务器上解压

```bash
ssh root@YOUR_SERVER_IP
cd /root
tar -xzf cliproxy-feishu-monitor-linux-x86_64.tar.gz
cd cliproxy-feishu-monitor-linux-x86_64
```

### 4. 生成运行配置

```bash
cp local.runtime.json.example local.runtime.json
```

或者直接使用仓库里的服务器模板：

```bash
cp local.runtime.server.json.example local.runtime.json
```

然后编辑：

```bash
nano local.runtime.json
```

至少改这几个字段：
- `management_key`
- `feishu_webhook`
- `feishu_secret`

### 5. 先手动跑一轮

```bash
bash run-once.sh --dry-run
```

确认输出正常后，真实推送一轮：

```bash
bash run-once.sh
```

### 6. 安装为 systemd 服务

```bash
bash install-service.sh
```

### 7. 查看状态和日志

```bash
bash service-status.sh
bash service-logs.sh
```

如果你不用 release 包，而是直接在源码目录部署，也可以继续看下面的“服务器部署”章节。

## 服务器部署

### 方式一：源码目录直接安装

```bash
cp local.runtime.json.example local.runtime.json
# 填好配置
bash deploy/install.sh
```

### 方式二：用离线发布包安装

```bash
tar -xzf cliproxy-feishu-monitor-linux-x86_64.tar.gz
cd cliproxy-feishu-monitor-linux-x86_64
cp local.runtime.json.example local.runtime.json
# 填好配置
bash install-service.sh
```

---

## 服务命令

安装完成后常用命令：

```bash
systemctl status cliproxy-feishu-monitor
journalctl -u cliproxy-feishu-monitor -f
```

如果你在 release 包目录里：

```bash
bash run-once.sh
bash service-status.sh
bash service-logs.sh
```

---

## 测试

```bash
go test ./...
```

---

## 说明

服务会把最近一次成功汇总结果写入 `state_path`，用于：
- 判断下次汇总是否到期
- 判断下次心跳是否到期
- 心跳里携带最近一次成功汇总摘要
- 连续失败时发送异常提醒


---

## 自动发版

现在仓库已经支持 **push tag 自动发 GitHub Release**。

### 本地发版脚本

```bash
bash deploy/release.sh v0.1.2
```

这个脚本会自动：
- 运行 `go test ./...`
- 构建 release tar 包
- 创建 git tag
- push tag 到 GitHub

随后 GitHub Actions 会自动：
- 再跑一次测试
- 构建 Linux release tar 包
- 创建对应 GitHub Release
- 上传 `cliproxy-feishu-monitor-linux-x86_64.tar.gz`

### 手动触发 Actions

你也可以在 GitHub Actions 页面手动触发 `release` 工作流，并填写版本号，例如：

```text
v0.1.2
```

---

## 服务器专用配置成品模板

仓库里现在有两份服务器模板：

- `local.runtime.server.json.example`
- `examples/local.runtime.server.changdaye.json.example`

如果你的服务器就是当前这套 CPA 地址，推荐直接：

```bash
cp examples/local.runtime.server.changdaye.json.example local.runtime.json
```

然后只改：
- `management_key`
- `feishu_webhook`
- `feishu_secret`

如果你是用 release 包部署，解压后同样会带上这份模板。
