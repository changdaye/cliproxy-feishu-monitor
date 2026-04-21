# Contributing

感谢你关注 `cliproxy-feishu-monitor`。

本项目当前重点是：
- 稳定获取 CLIProxyAPI 中 Codex 账号状态
- 稳定推送飞书消息
- 保持部署流程简单、可复用、低依赖

在提交改动前，请先阅读以下约定。

## 开发环境

要求：
- Go 1.25+
- Linux/macOS shell 环境

常用命令：

```bash
go test ./...
go build -o dist/cliproxy-feishu-monitor .
bash deploy/build-release.sh
```

## 提交前要求

至少完成以下检查：

1. `go test ./...` 通过
2. 如有构建相关改动，确认 `go build` 通过
3. 如有部署相关改动，至少检查相关 shell 脚本语法
4. 不要提交真实密钥、真实运行配置、真实状态文件

## 改动原则

- 优先做最小改动
- 不要顺手做无关重构
- 公开文档中不要写真实服务器 IP、真实 webhook、真实管理密钥
- 若修改部署行为，请同步更新 README 和相关示例配置
- 若修改定时策略、消息格式、部署方式，请同步更新 `docs/requirements.md`

## Pull Request 建议内容

请在 PR 中说明：
- 改了什么
- 为什么改
- 如何验证
- 是否有兼容性影响

