# 部署说明

这个项目的部署目标是“能公开演示的 MVP”，不是完整企业级高可用平台。生产风格 Compose 会把 MySQL、Redis、Qdrant、Prometheus 放在 Docker 内部网络，只通过 Caddy 暴露 Go Gateway。

## 1. 准备环境变量

```powershell
Copy-Item .env.prod.example .env.prod
```

编辑 `.env.prod`：

- `GATEWAY_API_KEY`：设置为足够长的随机字符串，用于保护 `/v1/*` 接口。
- `MYSQL_ROOT_PASSWORD`、`MYSQL_PASSWORD`、`REDIS_PASSWORD`：替换为真实密码。
- `LLM_API_KEY`：填写 DeepSeek 或其他 OpenAI 兼容供应商的 Key。
- `PUBLIC_DOMAIN`：有域名时填写域名，没有域名可以先用服务器 IP 或 `localhost` 测试。
- `CORS_ALLOW_ORIGINS`：填写前端来源，不建议生产环境使用 `*`。

不要提交 `.env.prod`。

## 2. 启动服务

```powershell
docker compose --env-file .env.prod -f deployments/docker-compose.prod.yml up -d --build
```

查看状态：

```powershell
docker compose --env-file .env.prod -f deployments/docker-compose.prod.yml ps
```

## 3. 验证接口

```powershell
$headers = @{ Authorization = "Bearer <GATEWAY_API_KEY>" }
Invoke-RestMethod http://localhost/health
Invoke-RestMethod http://localhost/v1/tools -Headers $headers
```

如果已经绑定域名，把 `http://localhost` 换成你的域名。

## 4. 网络暴露原则

公网只暴露：

- `80`
- `443`

不要把 MySQL、Redis、Qdrant 端口暴露到公网。它们只应该被同一个 Docker 网络内的 app 访问。

## 5. 运行状态

- `/metrics` 暴露 Prometheus 指标，由 Compose 内部的 Prometheus 抓取。
- 工具调用审计、策略拒绝、会话和消息记录写入 MySQL。
- Redis 用于限流。
- Qdrant 保存知识库向量和来源元数据。

## 6. 最小安全检查

- [ ] 已设置 `GATEWAY_API_KEY`。
- [ ] `.env.prod` 没有提交到 GitHub。
- [ ] MySQL、Redis、Qdrant 端口没有公开暴露。
- [ ] 生产环境 `CORS_ALLOW_ORIGINS` 不是 `*`。
- [ ] `RATE_LIMIT_FAIL_CLOSED=true`。
- [ ] 大模型供应商后台已配置额度或限额。
- [ ] 服务器安全组只开放必要端口。
