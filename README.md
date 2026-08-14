# Sakura 的快乐小屋

一个可扩展的个人记录与小屋社交应用。后端使用 Go、GORM、外部 PostgreSQL 和外部 Redis，前端使用 React、TypeScript 与 Vite；生产环境只运行一个应用容器。

## 快速启动

1. 准备宿主机或可访问网络中的 PostgreSQL 和 Redis。
2. 创建本地配置：

   ```bash
   cp config.example.yaml config.yaml
   cp .env.example .env
   ```

3. 编辑 `config.yaml` 中的 PostgreSQL、Redis 地址，并在 `.env` 中填写数据库密码。Redis 没有密码时保留 `SAKURA_HOME_REDIS_PASSWORD=`；数据库首次没有管理员时，还需设置至少 12 个字符的 `SAKURA_HOME_ADMIN_PASSWORD`。
4. 启动唯一的应用容器：

   ```bash
   docker compose up -d --build
   ```

5. 打开 <http://localhost:13888>。

Compose 不会启动 PostgreSQL、Redis 或 Node 容器。Node 仅用于 Docker 多阶段构建中的 React 编译，最终镜像中只有 Go 应用。

## 项目结构

```text
cmd/server/              Go 进程入口、HTTP handler、嵌入的前端产物
internal/domain/         领域模型
internal/repository/     GORM 仓储
internal/service/        业务服务
internal/platform/       PostgreSQL、Redis、SMTP 等基础设施
internal/config/         配置加载与校验
web/src/app/             React 应用组合与壳层
web/src/features/        记录、认证、社交、游戏等功能模块
web/src/shared/          API、类型和通用工具
docs/ARCHITECTURE.md     依赖方向与扩展约定
```

详细说明见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

## 本地开发

后端：

```bash
export SAKURA_HOME_DATABASE_PASSWORD='your-database-password'
export SAKURA_HOME_REDIS_PASSWORD='your-redis-password'
go run ./cmd/server
```

前端开发服务器：

```bash
cd web
npm ci
npm run dev
```

Vite 默认通过代理访问 `http://127.0.0.1:13888/api`。生产构建：

```bash
cd web && npm run build
go build ./cmd/server
```

## 持久化与安全

- 记录、用户、关系和系统设置保存在 PostgreSQL。
- 登录会话和鉴权限流保存在 Redis，并自动按 TTL 过期。
- 附件保存在 `storage.upload_dir`，容器默认挂载 `./data/uploads`。
- 应用面向公网时应启用 HTTPS、设置 `auth.cookie_secure: true`，并配置强数据库/Redis密码。

项目采用 [Apache License 2.0](LICENSE)。
