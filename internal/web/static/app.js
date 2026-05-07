const state = {
  baseUrl: window.location.origin,
  apiKey: window.localStorage.getItem("gatewayApiKey") || "",
};

const els = {
  baseUrl: document.querySelector("#base-url"),
  apiKey: document.querySelector("#api-key"),
  healthDot: document.querySelector("#health-dot"),
  healthText: document.querySelector("#health-text"),
  toolsList: document.querySelector("#tools-list"),
  timeline: document.querySelector("#timeline"),
  docContent: document.querySelector("#doc-content"),
  searchQuery: document.querySelector("#search-query"),
  calcExpression: document.querySelector("#calc-expression"),
  blockedSql: document.querySelector("#blocked-sql"),
  logKind: document.querySelector("#log-kind"),
};

function init() {
  els.baseUrl.value = state.baseUrl;
  els.apiKey.value = state.apiKey;
  bind("#btn-save-key", saveConfig);
  bind("#btn-health", checkHealth);
  bind("#btn-tools", loadTools);
  bind("#btn-index", indexDocument);
  bind("#btn-search", searchKnowledgeBase);
  bind("#btn-calc", runCalculator);
  bind("#btn-reject", triggerPolicyReject);
  bind("#btn-async", submitAsyncLogs);
  bind("#btn-audit-tools", () => listAudit("/v1/audit/tool-calls?limit=10", "工具调用审计"));
  bind("#btn-audit-rejects", () => listAudit("/v1/audit/policy-rejects?limit=10", "策略拒绝记录"));
  bind("#btn-clear", () => {
    els.timeline.innerHTML = "";
  });
  bind("#btn-run-all", runAll);
  checkHealth();
  loadTools();
}

function bind(selector, fn) {
  const el = document.querySelector(selector);
  el.addEventListener("click", () => withBusy(el, fn));
}

async function withBusy(el, fn) {
  el.disabled = true;
  try {
    await fn();
  } finally {
    el.disabled = false;
  }
}

function saveConfig() {
  state.baseUrl = trimRightSlash(els.baseUrl.value || window.location.origin);
  state.apiKey = els.apiKey.value.trim();
  window.localStorage.setItem("gatewayApiKey", state.apiKey);
  addEvent("配置已保存", { base_url: state.baseUrl, api_key: state.apiKey ? "configured" : "empty" }, "success");
}

async function checkHealth() {
  state.baseUrl = trimRightSlash(els.baseUrl.value || window.location.origin);
  try {
    const data = await request("/health", { auth: false });
    setHealth(true, "healthy");
    addEvent("健康检查成功", data, "success");
  } catch (err) {
    setHealth(false, "unhealthy");
    addEvent("健康检查失败", errorPayload(err), "error");
  }
}

async function loadTools() {
  try {
    const data = await request("/v1/tools");
    const tools = Array.isArray(data.data) ? data.data : [];
    els.toolsList.innerHTML = tools
      .map(
        (tool) => `
          <div class="tool-item">
            <strong>${escapeHtml(tool.name)}</strong>
            <span>${escapeHtml(tool.permission)} · timeout ${escapeHtml(String(tool.timeout_ms))}ms · async ${tool.async}</span>
          </div>
        `,
      )
      .join("");
    addEvent("工具列表已刷新", data, "success");
  } catch (err) {
    addEvent("工具列表获取失败", errorPayload(err), "error");
  }
}

async function indexDocument() {
  const body = {
    title: "Gateway Demo Guide",
    source_type: "demo",
    source_path: "demo/gateway-guide.md",
    content: els.docContent.value,
  };
  const data = await request("/v1/documents", { method: "POST", body });
  addEvent("文档已写入知识库", data, "success");
}

async function searchKnowledgeBase() {
  const body = {
    user_id: "demo-user",
    tool_name: "search_knowledge_base",
    input: {
      query: els.searchQuery.value,
      top_k: 5,
    },
  };
  const data = await request("/v1/tools/execute", { method: "POST", body });
  addEvent("Qdrant 语义检索完成", data, "success");
}

async function runCalculator() {
  const body = {
    user_id: "demo-user",
    tool_name: "calculator",
    input: {
      expression: els.calcExpression.value,
    },
  };
  const data = await request("/v1/tools/execute", { method: "POST", body });
  addEvent("calculator 执行完成", data, "success");
}

async function triggerPolicyReject() {
  const body = {
    user_id: "admin",
    tool_name: "query_mysql_readonly",
    input: {
      sql: els.blockedSql.value,
      api_key: "demo-secret-value",
    },
  };
  try {
    const data = await request("/v1/tools/execute", { method: "POST", body });
    addEvent("SQL 请求执行完成", data, "success");
  } catch (err) {
    addEvent("SQL 策略拒绝已触发", errorPayload(err), "error");
  }
}

async function submitAsyncLogs() {
  const body = {
    user_id: "demo-user",
    tool_name: "query_logs",
    async: true,
    input: {
      kind: els.logKind.value,
      limit: 10,
    },
  };
  const submitted = await request("/v1/tools/execute", { method: "POST", body });
  addEvent("异步任务已提交", submitted, "success");
  const taskID = submitted?.data?.task_id;
  if (!taskID) {
    return;
  }
  await sleep(500);
  const status = await request(`/v1/async-tasks/${encodeURIComponent(taskID)}`);
  addEvent("异步任务状态", status, status?.data?.status === "failed" ? "error" : "success");
}

async function listAudit(path, title) {
  try {
    const data = await request(path);
    addEvent(title, data, "success");
  } catch (err) {
    addEvent(`${title}失败`, errorPayload(err), "error");
  }
}

async function runAll() {
  await checkHealth();
  await loadTools();
  await indexDocument();
  await searchKnowledgeBase();
  await runCalculator();
  await triggerPolicyReject();
  await submitAsyncLogs();
  await listAudit("/v1/audit/policy-rejects?limit=10", "策略拒绝记录");
}

async function request(path, opts = {}) {
  const method = opts.method || "GET";
  const headers = { Accept: "application/json" };
  if (opts.body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (opts.auth !== false && state.apiKey) {
    headers.Authorization = `Bearer ${state.apiKey}`;
  }
  const resp = await fetch(`${state.baseUrl}${path}`, {
    method,
    headers,
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
  });
  const text = await resp.text();
  let payload = null;
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch {
      payload = { raw: text };
    }
  }
  if (!resp.ok) {
    const err = new Error(payload?.message || `HTTP ${resp.status}`);
    err.status = resp.status;
    err.payload = payload;
    throw err;
  }
  return payload;
}

function addEvent(title, payload, kind = "") {
  const item = document.createElement("article");
  item.className = `event ${kind}`;
  const time = new Date().toLocaleTimeString();
  item.innerHTML = `
    <div class="event-head">
      <span>${escapeHtml(title)}</span>
      <time>${escapeHtml(time)}</time>
    </div>
    <pre>${escapeHtml(JSON.stringify(payload, null, 2))}</pre>
  `;
  els.timeline.prepend(item);
}

function setHealth(ok, text) {
  els.healthDot.className = `dot ${ok ? "ok" : "bad"}`;
  els.healthText.textContent = text;
}

function errorPayload(err) {
  return {
    status: err.status || "network_error",
    message: err.message,
    response: err.payload || null,
  };
}

function trimRightSlash(value) {
  return value.replace(/\/+$/, "");
}

function sleep(ms) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

init();
