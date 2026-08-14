# 项目架构

项目采用单应用容器部署：React 在镜像构建阶段编译为静态资源，Go 在运行阶段同时提供 API 与前端页面。PostgreSQL 和 Redis 是外部基础设施，不在 Compose 中创建额外容器。

## 后端边界

```text
cmd/server/                 进程入口、路由、HTTP 请求与响应映射
internal/config/            配置加载、环境变量覆盖与校验
internal/domain/            领域模型与对外视图
internal/repository/        GORM 数据访问，只负责持久化
internal/service/           业务规则、事务编排、密码与附件逻辑
internal/platform/database/ PostgreSQL/GORM 连接与幂等结构迁移
internal/platform/cache/    Redis 会话、限流与后续缓存能力
internal/platform/mailer/   SMTP 基础设施实现
```

依赖方向为 `transport -> service -> repository/domain`。业务服务不依赖 HTTP，仓储不处理业务提示文本。新增领域时，依次增加 `domain` 模型、`repository`、`service` 和 `cmd/server` handler。

会话只保存在 Redis，并使用 `redis.key_prefix` 统一命名空间和 `auth.session_ttl_hours` 设置 TTL。PostgreSQL 中旧的 `sessions` 表不会再被新代码读写，可在确认无需回滚旧版本后单独安排数据库清理。

## 前端边界

```text
web/src/app/          应用组合、导航与应用壳层
web/src/features/     按业务功能拆分的页面与交互
web/src/shared/       API 客户端、通用类型与格式化工具
web/src/styles.css    设计 token、组件样式与响应式规则
```

前端通过同源 `/api` 访问后端。Vite 开发服务器会把 `/api` 代理到 `127.0.0.1:13888`，生产资源由 Go 嵌入并提供。

## 配置策略

普通配置写入 `config.yaml`。数据库、Redis 密码等敏感值优先通过环境变量覆盖：

- `SAKURA_HOME_DATABASE_PASSWORD`
- `SAKURA_HOME_REDIS_PASSWORD`
- `SAKURA_HOME_ADMIN_USERNAME`
- `SAKURA_HOME_ADMIN_PASSWORD`

## 扩展约定

1. 不在 `cmd/server` 直接写 GORM 查询。
2. 不在 repository 中读取 HTTP 请求或写 HTTP 响应。
3. Redis key 必须经过 `internal/platform/cache` 统一加前缀。
4. 前端新功能放在独立 `features/<feature>`，`App.tsx` 只负责组合。
5. 数据库模型变更在 `internal/platform/database` 增加显式迁移语句，保持幂等并兼容已有数据；复杂数据回填单独编写迁移函数。
