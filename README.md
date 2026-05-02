# Go Safe Agent Gateway

一个面向 LLM Agent 的安全工具调用网关。核心思路是：大模型只负责生成工具调用意图，真正访问 MySQL、HTTP API、知识库和审计日志的动作，统一由 Go 网关完成注册、参数校验、策略判断、超时隔离、审计记录和指标采集。

这个项目定位是可部署、可演示的简历级 MVP，不追求企业级平台的完整复杂度，但保留了安全网关、RAG、审计、限流、OpenAPI、Docker Compose 和 Prometheus 这些能体现工程能力的关键链路。

## 核心能力

- Tool Registry：统一注册 `calculator`、`query_mysql_readonly`、`http_get`、`search_knowledge_base`、`query_logs` 等工具。
- Policy Engine：实现工具 allowlist、权限等级、SQL 只读校验、URL allowlist、防 SSRF、敏感字段脱敏和 Redis 限流。
- Tool Executor：提供执行超时、panic recovery、审计落库、Prometheus 指标和异步任务执行。
- Qdrant RAG：支持文档切分、embedding 抽象、向量写入、TopK 语义检索和来源元数据返回。
- MySQL 审计：持久化会话、消息、工具调用和策略拒绝记录。
- 部署闭环：提供本地 Compose、生产风格 Compose、Caddy 反向代理、Prometheus、OpenAPI 和 demo 脚本。

## 架构说明

```text
Client
  -> Gin Handler
  -> Agent Service
  -> LLM Tool Call Intent
  -> Tool Registry
  -> JSON Schema Validation
  -> Policy Engine
  -> Tool Executor
  -> Backend Tool
  -> MySQL Audit + Prometheus Metrics
  -> Response
```

更完整的 Mermaid 架构图见 [docs/architecture.md](docs/architecture.md)。

## 本地启动

复制环境变量模板：

```powershell
Copy-Item .env.example .env
```

只启动依赖服务，然后本地运行 Go 程序：

```powershell
docker compose --env-file .env -f deployments/docker-compose.yml up -d mysql redis qdrant prometheus
go run ./cmd/server
```

也可以直接启动完整容器栈：

```powershell
docker compose --env-file .env -f deployments/docker-compose.yml up -d --build
```

默认端口：

```text
App        http://127.0.0.1:8080
Prometheus http://127.0.0.1:9090
MySQL      127.0.0.1:13306
Redis      127.0.0.1:6379
Qdrant     http://127.0.0.1:6333
```

## API Key 和大模型配置

本地开发默认不开启网关鉴权，`GATEWAY_API_KEY` 可以留空。准备部署或公开访问时必须设置：

```text
GATEWAY_API_KEY=replace-with-a-long-random-key
```

之后访问 `/v1/*` 接口需要带上：

```http
Authorization: Bearer replace-with-a-long-random-key
```

默认 `.env.example` 使用 mock LLM 和 mock embedding，方便无付费 API Key 时本地演示：

```text
ENABLE_MOCK_LLM=true
ENABLE_MOCK_EMBEDDING=true
```

接入 DeepSeek 聊天接口时可以改成：

```text
ENABLE_MOCK_LLM=false
LLM_BASE_URL=https://api.deepseek.com/v1
LLM_API_KEY=your-provider-key
LLM_MODEL=deepseek-chat
```

如果当前供应商没有 OpenAI 兼容的 `/embeddings` 接口，建议继续保持 `ENABLE_MOCK_EMBEDDING=true`，否则 RAG 向量维度和模型输出需要一起调整。

## 常用接口

健康检查：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/health
```

查看工具列表：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/v1/tools
```

写入一篇知识库文档：

```powershell
$doc = @{
  title = "Gateway Guide"
  source_type = "manual"
  source_path = "docs/gateway-guide.md"
  content = "# Policy Engine`nThe gateway validates tool permission, SQL safety, URL allowlists, and rate limits before execution."
} | ConvertTo-Json

Invoke-RestMethod `
  -Uri http://127.0.0.1:8080/v1/documents `
  -Method Post `
  -ContentType "application/json" `
  -Body $doc
```

执行 Qdrant 知识库检索：

```powershell
$body = @{
  user_id = "user-1"
  tool_name = "search_knowledge_base"
  input = @{
    query = "policy permission rate limits"
    top_k = 3
  }
} | ConvertTo-Json -Depth 5

Invoke-RestMethod `
  -Uri http://127.0.0.1:8080/v1/tools/execute `
  -Method Post `
  -ContentType "application/json" `
  -Body $body |
ConvertTo-Json -Depth 10
```

## Demo 和压测

运行本地端到端 demo：

```powershell
.\scripts\demo.ps1
```

脚本会生成 `demo-output/`，里面包含 JSON 响应、transcript 和 HTML 报告。该目录已被 `.gitignore` 忽略，不应提交到 GitHub。

运行 k6 smoke test：

```powershell
k6 run scripts/k6-smoke.js
```

## 生产风格部署

复制生产环境变量模板：

```powershell
Copy-Item .env.prod.example .env.prod
```

修改 `.env.prod` 中的 API Key、数据库密码、Redis 密码、LLM Key、域名和 CORS 来源，然后启动：

```powershell
docker compose --env-file .env.prod -f deployments/docker-compose.prod.yml up -d --build
```

生产风格部署中只有 Caddy 对外暴露 `80/443`，MySQL、Redis、Qdrant、Prometheus 都留在 Docker 内部网络。详细说明见 [docs/deploy.md](docs/deploy.md)。

## 上传 GitHub 前检查

不要提交 `.env`、`.env.prod`、`demo-output/`、`.tmp/`、二进制文件和本地 IDE 配置。仓库只保留 `.env.example` 和 `.env.prod.example` 作为配置模板。

建议上传前执行：

```powershell
gofmt -w .
go test ./...
go vet ./...
docker compose --env-file .env.example -f deployments/docker-compose.yml config --quiet
docker compose --env-file .env.prod.example -f deployments/docker-compose.prod.yml config --quiet
```

OpenAPI 规格见 [docs/openapi.yaml](docs/openapi.yaml)。
