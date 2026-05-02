# 架构说明

本项目是一个可部署的 LLM Agent 安全工具调用网关。核心设计点是：LLM 不直接访问内部系统，只输出工具调用意图；Go Gateway 负责校验、授权、执行、审计和观测。

## 系统视图

```mermaid
flowchart LR
    User[User or External Agent] --> Caddy[Caddy Reverse Proxy]
    Caddy --> API[Go Safe Agent Gateway]

    API --> Auth[API Key Auth]
    API --> Router[Gin Router and Handlers]
    Router --> Service[Agent Service]

    Service --> LLM[OpenAI-compatible LLM or DeepSeek]
    Service --> Executor[Tool Executor]

    Executor --> Registry[Tool Registry]
    Executor --> Schema[JSON Schema Validation]
    Executor --> Policy[Policy Engine]
    Executor --> Audit[(MySQL Audit Logs)]
    Executor --> Metrics[Prometheus Metrics]

    Policy --> Redis[(Redis Rate Limit)]
    Policy --> Audit

    Executor --> Tools[Backend Tools]
    Tools --> MySQL[(MySQL Readonly Query)]
    Tools --> HTTP[Allowlisted HTTP APIs]
    Tools --> Qdrant[(Qdrant Vector Search)]
    Tools --> Logs[(Audit and Policy Logs)]

    API --> Prom[Prometheus Scrape /metrics]
```

## 工具执行链路

```mermaid
sequenceDiagram
    participant Client
    participant Handler
    participant Service
    participant LLM
    participant Executor
    participant Policy
    participant Tool
    participant Store as MySQL Audit

    Client->>Handler: POST /v1/agent/chat
    Handler->>Service: Bind request
    Service->>Store: Save user message
    Service->>LLM: Generate tool-call intent
    LLM-->>Service: {tool_name, input}
    Service->>Executor: Execute tool request
    Executor->>Executor: Registry lookup
    Executor->>Executor: JSON Schema validation
    Executor->>Policy: Check permissions, SQL, URL, rate limit
    Policy-->>Executor: allow or reject
    alt Allowed
        Executor->>Tool: Execute with context timeout
        Tool-->>Executor: Tool result
        Executor->>Store: Save audit log
        Service->>LLM: Generate final answer from tool result
        LLM-->>Service: Final answer
        Service->>Store: Save assistant message
        Service-->>Handler: Answer + tool result
    else Rejected
        Executor->>Store: Save policy rejection
        Executor-->>Service: Controlled error
    end
    Handler-->>Client: Standard JSON response
```

## RAG 链路

```mermaid
flowchart TD
    Doc[POST /v1/documents] --> Split[Markdown chunk splitter]
    Split --> Embed[Embedding generator]
    Embed --> Upsert[Qdrant upsert vectors]
    Split --> Meta[(MySQL document metadata)]

    Query[search_knowledge_base] --> QueryEmbed[Query embedding]
    QueryEmbed --> Search[Qdrant vector search]
    Search --> Result[Chunks with score and citation metadata]
```

## 部署视图

```mermaid
flowchart TB
    Internet[Internet or Internal Network] --> Caddy[Caddy :80/:443]
    Caddy --> App[Go Gateway :8080]
    Prometheus[Prometheus] --> App
    App --> MySQL[(MySQL)]
    App --> Redis[(Redis)]
    App --> Qdrant[(Qdrant)]
    App --> Provider[DeepSeek / OpenAI-compatible API]

    subgraph Private Docker Network
        App
        MySQL
        Redis
        Qdrant
        Prometheus
    end
```

## 简历表达重点

- LLM 不能直接访问数据库、Redis、Qdrant、HTTP API 或审计日志，所有能力都先封装成 Tool。
- 每次工具调用都会经过 registry lookup、JSON Schema validation、policy check、timeout、panic recovery、audit logging 和 metrics。
- Qdrant RAG 返回带 `document_id`、`source_path`、`chunk_index` 的检索结果，便于追踪来源。
- Redis 限流、MySQL 审计、Prometheus 指标和 Docker Compose 部署让项目具备可运行、可观测、可演示的完整闭环。
