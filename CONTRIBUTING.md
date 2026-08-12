# 贡献指南

## 基本流程

1. 先搜索现有 Issue 和 Pull Request。较大变更应先通过 Issue 确认范围。
2. 从最新的 `main` 创建单一目的的分支，例如 `fix/session-expiry`。
3. 完成单一目的的修改，并记录可复现的验证方式和结果。
4. 在本地运行质量检查后提交 Pull Request。
5. 回应审核意见；追加代码后必须重新审核。

禁止直接向 `main` 推送。维护者也必须通过 Pull Request 修改代码。

## 本地检查

```bash
gofmt -w ./cmd ./internal
go mod tidy
go vet ./...
go build ./...
```

涉及 Docker 时还应运行：

```bash
docker build --file Dockerfile --tag sakura-happy-cottage:local .
```

## 审核标准

审核者应按 [`docs/CODE_REVIEW.md`](docs/CODE_REVIEW.md) 检查并留下可追溯结论。

Pull Request 必须满足以下条件才能合并：

- 所有必需 CI 检查通过，没有跳过或忽略失败。
- 至少由一名非作者维护者批准；成熟维护团队应要求两名批准者。
- 新提交会使旧批准失效，最后一次可审核推送必须由其他人批准。
- 所有审核对话均已解决，`CODEOWNERS` 指定的负责人已批准。
- 不包含无关重构、生成物、本地 IDE 文件、真实配置、秘密或用户数据。
- API、数据库、认证、授权、上传和邮件变更明确说明兼容性及安全影响。

默认使用 squash merge，保持 `main` 线性。提交和 Pull Request 标题应简短说明行为变化。

## 安全要求

- 不要把真实 `config.yaml`、`.env`、数据库目录或附件加入 Git。
- 不要在来自 fork 的工作流中使用 Secrets，也不要使用 `pull_request_target` 执行贡献者代码。
- 依赖更新必须通过 Dependency Review 和漏洞扫描。
- 安全漏洞按 `SECURITY.md` 私下报告。
