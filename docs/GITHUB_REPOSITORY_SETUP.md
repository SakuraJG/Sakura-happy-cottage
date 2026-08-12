# GitHub 仓库上线清单

本清单用于首次创建公开仓库。完成每一项后再开放可见性。

## 1. 首次推送前

- [ ] 确认 `git status --ignored` 中 `.idea/`、`config.yaml`、`.env`、`data/` 均为 ignored。
- [ ] 确认 `git ls-files .idea config.yaml .env data` 无输出。
- [ ] 运行 `go build ./...`、`go vet ./...` 和 `go mod verify`。
- [ ] 运行 `govulncheck ./...`，必须显示没有影响当前代码的已知漏洞。
- [ ] 使用 Gitleaks 或等效工具扫描完整待提交内容。
- [ ] 添加已经明确选择的开源 `LICENSE`。
- [ ] 在 `.github/CODEOWNERS` 填入真实且拥有 Write 权限的 GitHub 用户或团队。

仓库没有 `LICENSE` 时只是源码公开，不是完整的开源授权。不要用占位许可证或未经确认的版权人信息。

## 2. 创建仓库

建议先在 GitHub Organization 下创建空仓库，方便用团队而不是个人账号管理权限。创建时：

- Visibility 先选 **Private**；
- 不初始化 README、`.gitignore` 或 LICENSE；
- 不添加任何真实部署 Secrets；
- `Admin` 仅授予 1-2 名负责人，审核者使用 `Write`，Issue 管理人员使用 `Triage`。

本地完成首个提交后添加远程并推送：

```bash
git remote add origin git@github.com:OWNER/sakura-happy-cottage.git
git push -u origin main
```

## 3. Actions

进入 `Settings > Actions > General`：

- Actions permissions：只允许 GitHub 官方 Action 和明确批准的 Action；
- 开启 **Require actions to be pinned to a full-length commit SHA**；
- Fork PR workflow approval：选择 **Require approval for all external contributors**；
- Workflow permissions：选择只读 `contents` / `packages`；
- 关闭 **Allow GitHub Actions to create and approve pull requests**；
- 不给来自 fork 的 workflow 发送写 Token 或 Secrets；
- 公共仓库不使用连接生产网络的 self-hosted runner。

首次推送后确认以下 checks 均能成功：`quality`、`vulnerability`、`container`。仓库转为公开后，再通过一个仅修改 `go.mod` 的验证 PR 确认 `dependency-review` 成功。

## 4. Security

进入 `Settings > Code security and analysis`，启用：

- Dependency graph；
- Dependabot alerts；
- Dependabot security updates；
- Secret scanning；
- Push protection；
- CodeQL default setup；
- Private vulnerability reporting。

将 `Security` 页签中的未处理高危或严重告警视为发布阻断项。

## 5. Main Ruleset

进入 `Settings > Rules > Rulesets > New branch ruleset`，创建 `protect-main`：

- Enforcement status：`Active`；
- Target branches：`Default branch`；
- Bypass list：留空；
- Restrict deletions：开启；
- Block force pushes：开启；
- Require linear history：开启；
- Require a pull request before merging：开启；
- Required approvals：成熟团队设为 `2`，只有一名独立审核者时暂设 `1`；
- Dismiss stale pull request approvals when new commits are pushed：开启；
- Require review from Code Owners：开启；
- Require approval of the most recent reviewable push：开启；
- Require conversation resolution before merging：开启；
- Allowed merge method：仅 `Squash`；
- Require status checks：开启 strict/up-to-date，并添加 `quality`、`vulnerability`、`container`、`dependency-review` 和 CodeQL 检查；
- Do not allow bypassing / 管理员同样受规则约束：开启。

Pull Request 作者不能批准自己的变更。若当前只有仓库所有者一人，不要通过给管理员永久 bypass 来伪装审核；应先邀请至少一名可信审核者，或接受在此之前 PR 无法合并。

## 6. 合并设置

进入 `Settings > General > Pull Requests`：

- 仅启用 **Allow squash merging**；
- 开启自动删除 head branches；
- 可启用 Auto-merge，但不能削弱 Ruleset；
- 不启用“失败后仍可合并”一类旁路自动化。

## 7. 公开前复核

- [ ] README 不含内网地址、真实邮箱、账号、密码或机器路径。
- [ ] Git 历史没有秘密；发现秘密时先撤销凭据，再清理历史。
- [ ] LICENSE、SECURITY、CONTRIBUTING、CODEOWNERS 可被 GitHub 正确识别。
- [ ] 从外部账号提交测试 PR，确认 fork workflow 需维护者批准且拿不到 Secrets。
- [ ] 尝试直接推送、force push、未审核合并和带失败 check 合并，均应被阻止。
- [ ] 开启公开可见性后立即检查 Security、Actions 和 Rules 页面状态。
