# Sakura的快乐小屋

一个以事项记录为主要功能的个人小屋应用。界面、API、认证、PostgreSQL 数据访问、邮件确认和文件存储都在同一个 Go 服务中，连接宿主机已有的 PostgreSQL。

## 快速启动

1. 创建本地配置和环境变量文件：

   ```bash
   cp config.example.yaml config.yaml
   cp .env.example .env
   ```

2. 编辑 `.env`，填写数据库密码。首次启动且数据库中尚无管理员时，还必须设置一个至少 12 个字符的 `SAKURA_HOME_ADMIN_PASSWORD`。
3. 执行：

   ```bash
   docker compose up -d --build
   ```

4. 本机打开 <http://localhost:13888>，局域网设备使用 `http://本机局域网IP:13888`。

停止服务：

```bash
docker compose down
```

首次启动会在现有数据库中自动创建数据表，不会创建 PostgreSQL 实例。默认管理员用户名为 `admin`，但应用不提供默认密码，也会拒绝常见占位密码。管理员创建并修改密码后，可以从 `.env` 删除 `SAKURA_HOME_ADMIN_PASSWORD`。修改配置后，使用 `docker compose restart` 重新加载。Compose 将 `13888` 发布到宿主机所有网卡；如果修改 `server.port`，需要同步修改端口映射。

## 持久化目录

- `./data/uploads`：截图与文件附件目录，不进入容器临时层。
- `config.yaml`：本地应用配置，不进入 Git；系统功能开关与 SMTP 设置保存在 PostgreSQL。
- `.env`：本地敏感环境变量，不进入 Git。

容器以 UID/GID `10001:10001` 的非 root 用户运行。在 Linux 主机首次启动前，确保 `./data/uploads` 可由该用户写入：

```bash
sudo chown -R 10001:10001 ./data/uploads
```

示例配置让容器通过 `host.docker.internal:5432` 访问宿主机 PostgreSQL。数据库名、用户和地址可在 `config.yaml` 修改，密码应通过 `SAKURA_HOME_DATABASE_PASSWORD` 注入。

## 功能

- 用户注册、账户名或已确认邮箱登录、退出登录；账户名用于登录，用户名用于公开展示。
- 每个用户拥有稳定数字 UID，可编辑用户名和个人简介，并通过右上角头像进入个人空间。
- 通过 UID 或用户名搜索用户，支持关注与取消关注；双方互相关注后自动出现在好友列表。
- bcrypt 密码哈希、服务端会话、修改密码和邮件找回密码。
- 绑定或更换邮箱，并通过一次性邮件链接确认。
- 管理员系统页面：开放注册、邮箱确认、密码找回、公开地址及 SMTP 设置。
- 创建记录：标题、具体描述、多个截图或文件附件。
- 进行中 / 已完成状态切换，自动记录创建时间和完成时间。
- 关键词搜索、状态筛选、日期范围筛选、时间排序、编辑和删除。
- 顶部导航提供独立的小游戏目录，后续游戏可沿 `/games` 路径逐步扩展。
- 附件保存在本地目录，支持预览图片和下载其他文件。

升级前没有用户归属的记录会自动分配给管理员账号。之后每个用户只能访问自己的记录和附件。

## 邮件配置

使用管理员账号登录，打开右上角账户设置，在“系统设置”中填写 SMTP 参数并启用邮件服务。公开访问地址必须是收件设备能够访问的地址；局域网使用时通常为 `http://本机局域网IP:13888`。SMTP 密码只写入数据库，不会通过管理接口回显。

支持 `none`、`starttls` 和 `tls` 三种 SMTP 加密模式。管理员可以分别启用邮箱确认和找回密码；两者启用时必须同时启用 SMTP。邮箱确认链接默认 24 小时有效，重置密码链接默认 30 分钟有效，时效可在 `config.yaml` 的 `auth` 部分配置。

## 局域网安全

应用允许局域网访问，请修改默认管理员密码、使用强密码并仅在可信网络运行。若通过公网或反向代理访问，应启用 HTTPS、将 `auth.cookie_secure` 改为 `true`，并在管理员系统设置中把公开访问地址改为 HTTPS 地址。

## 本地开发

准备本地 PostgreSQL，将 `config.yaml` 中的连接地址和 `storage.upload_dir` 改为本机路径，并设置密码环境变量：

```bash
go mod download
export SAKURA_HOME_DATABASE_PASSWORD='your-local-password'
export SAKURA_HOME_ADMIN_PASSWORD='your-first-start-password'
go run ./cmd/server
```

运行检查：

```bash
go vet ./...
go build ./...
```

## 参与贡献

所有代码变更必须通过 Pull Request。提交前请阅读 [CONTRIBUTING.md](CONTRIBUTING.md)；安全问题不要创建公开 Issue，请按照 [SECURITY.md](SECURITY.md) 私下报告。

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 开源。
