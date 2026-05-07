# 部署说明

这个项目的部署目标是“能公开演示的 MVP”，不是完整企业级高可用平台。生产风格 Compose 会把 MySQL、Redis、Qdrant、Prometheus 放在 Docker 内部网络，只通过 Caddy 暴露 Go Gateway。

## 1. 腾讯云安全组

公网只开放：

- `22`：SSH 登录服务器。
- `80`：HTTP 访问。
- `443`：HTTPS 访问。

不要开放：

- `3306`：MySQL。
- `6379`：Redis。
- `6333/6334`：Qdrant。
- `9090`：Prometheus。

## 2. 服务器依赖

推荐 Ubuntu 22.04 或 24.04。

```bash
sudo apt update
sudo apt install -y ca-certificates curl git
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo usermod -aG docker $USER
```

执行完 `usermod` 后重新登录 SSH，或者临时执行：

```bash
newgrp docker
```

验证：

```bash
docker version
docker compose version
```

## 3. 拉取代码

```bash
git clone https://github.com/<your-name>/go-safe-agent-gateway.git
cd go-safe-agent-gateway
```

如果仓库还没上传 GitHub，也可以先用 `scp` 或腾讯云控制台上传压缩包，但正式展示建议使用 GitHub 仓库拉取。

## 4. 配置生产环境变量

```bash
cp .env.prod.example .env.prod
nano .env.prod
```

必须修改：

```text
PUBLIC_DOMAIN=你的域名
GATEWAY_API_KEY=一个足够长的随机字符串
MYSQL_ROOT_PASSWORD=强密码
MYSQL_PASSWORD=强密码
REDIS_PASSWORD=强密码
LLM_API_KEY=你的 DeepSeek 或 OpenAI 兼容供应商 Key
CORS_ALLOW_ORIGINS=http://你的域名或服务器公网IP
```

如果已经备案并绑定域名，`PUBLIC_DOMAIN` 写域名，Caddy 会自动申请 HTTPS 证书。

如果暂时没有域名，先用公网 IP 做 HTTP 演示：

```text
PUBLIC_DOMAIN=:80
CORS_ALLOW_ORIGINS=http://服务器公网IP
```

不要提交 `.env.prod`。

## 5. 启动生产栈

```bash
docker compose --env-file .env.prod -f deployments/docker-compose.prod.yml up -d --build
```

查看状态：

```bash
docker compose --env-file .env.prod -f deployments/docker-compose.prod.yml ps
```

查看日志：

```bash
docker compose --env-file .env.prod -f deployments/docker-compose.prod.yml logs -f app
```

## 6. 验证接口

假设你的地址是：

```text
http://你的域名或公网IP
```

健康检查不需要 API Key：

```bash
curl http://你的域名或公网IP/health
```

Demo 页面：

```text
http://你的域名或公网IP/demo
```

`/v1/*` 需要 API Key：

```bash
curl http://你的域名或公网IP/v1/tools \
  -H "Authorization: Bearer 你的GATEWAY_API_KEY"
```

`/metrics` 也需要 API Key，避免公网裸露指标：

```bash
curl http://你的域名或公网IP/metrics \
  -H "Authorization: Bearer 你的GATEWAY_API_KEY"
```

Prometheus 容器仍然会在 Docker 内部网络直接抓取 `app:8080/metrics`。

## 7. 更新部署

以后代码更新后：

```bash
git pull
docker compose --env-file .env.prod -f deployments/docker-compose.prod.yml up -d --build
```

## 8. 最小安全检查

- [ ] 已设置 `GATEWAY_API_KEY`。
- [ ] `.env.prod` 没有提交到 GitHub。
- [ ] MySQL、Redis、Qdrant、Prometheus 端口没有公开暴露。
- [ ] `CORS_ALLOW_ORIGINS` 不是 `*`。
- [ ] `RATE_LIMIT_FAIL_CLOSED=true`。
- [ ] 大模型供应商后台已配置额度或限额。
- [ ] 腾讯云安全组只开放 `22/80/443`。
