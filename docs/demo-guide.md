# Demo 演示指南

这个页面用于面试、简历项目截图和本地验收。它不是单独的前端项目，而是被 Go embed 打进服务二进制，启动后直接访问：

```text
http://127.0.0.1:8080/demo
```

## 本地启动

```powershell
Copy-Item .env.example .env
docker compose --env-file .env -f deployments/docker-compose.yml up -d mysql redis qdrant prometheus
go run ./cmd/server
```

打开浏览器：

```text
http://127.0.0.1:8080/demo
```

如果 `.env` 配置了 `GATEWAY_API_KEY`，需要在页面左侧填写同一个 Key。

## 推荐演示顺序

1. 点击“检查”，确认 `/health` 正常。
2. 点击“刷新工具”，展示 Tool Registry 中的工具元数据。
3. 点击“写入 Qdrant”，把示例 Markdown 文档切分并写入知识库。
4. 点击“执行 search_knowledge_base”，展示 Qdrant TopK 语义检索和来源元数据。
5. 点击“执行 calculator”，展示普通工具调用链路。
6. 点击“触发 SQL 策略拒绝”，展示 Policy Engine 拒绝危险 SQL，同时敏感字段会被脱敏记录。
7. 点击“提交 query_logs 异步任务”，展示异步工具执行和任务状态查询。
8. 点击“拒绝记录”或“工具审计”，展示 MySQL 审计数据。

## 适合截图的位置

- `/demo` 首屏：展示这是一个可操作的安全网关控制台。
- 工具列表：展示工具注册、权限等级、超时时间和异步能力。
- Qdrant 检索结果：展示 `document_id`、`source_path`、`chunk_index`、`score`。
- SQL 拒绝结果：展示安全策略不是文档描述，而是真正参与执行链路。
- Prometheus：访问 `http://127.0.0.1:9090`，搜索 `tool_calls_total` 或 `http_requests_total`。

## 线上演示注意事项

- 线上必须设置 `GATEWAY_API_KEY`。
- 不要公开 MySQL、Redis、Qdrant 端口。
- 如果页面部署在同域名下，`Base URL` 保持默认即可。
- 如果页面访问的是另一个 API 域名，需要正确配置 `CORS_ALLOW_ORIGINS`。
