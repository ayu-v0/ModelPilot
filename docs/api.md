# ModelPilot HTTP API

这是不依赖前端的 Web API 服务，保留原有 CLI。API 版本前缀为 `/api/v1`。

## 部署

### Docker Compose（推荐）

在项目根目录创建 `.env`（不要提交到 Git）：

```dotenv
MODELPILOT_API_KEY=replace-with-a-long-random-secret
MODELPILOT_ALLOWED_HOSTS=api.openai.com,inference.example.com
MODELPILOT_MAX_JOBS=4
MODELPILOT_JOB_TIMEOUT=30m
```

```bash
docker compose up -d --build
curl http://localhost:8080/healthz
```

Compose 默认只绑定本机 `127.0.0.1:8080`。远程调用请通过 Nginx/Caddy 等 HTTPS 反向代理暴露该端口；不要在公网使用明文 HTTP 传递密钥。结果保存到命名卷 `modelpilot-data`，容器重建不丢失；不要使用 `docker compose down -v`，否则会删除数据。

### Go 直接运行

```bash
export MODELPILOT_API_KEY='replace-with-a-long-random-secret'
export MODELPILOT_ALLOWED_HOSTS='api.openai.com,inference.example.com'
go run ./cmd/modelpilot-server
# 或构建部署
go build -o modelpilot-server ./cmd/modelpilot-server
./modelpilot-server
```

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `MODELPILOT_API_KEY` | 无，必须设置 | 本服务的访问密钥，不是上游模型密钥 |
| `MODELPILOT_ADDR` | `:8080` | HTTP 监听地址 |
| `MODELPILOT_DATA_DIR` | `./data/jobs` | 任务与结果存储目录 |
| `MODELPILOT_ALLOWED_HOSTS` | 空 | 允许调用的模型端点主机名，逗号分隔，精确匹配，不带协议、端口或路径 |
| `MODELPILOT_MAX_JOBS` | `4` | 最大同时运行任务数，1..32 |
| `MODELPILOT_JOB_TIMEOUT` | `30m` | 单任务最长运行时间，例如 `10m`、`1h` |

除 `/healthz` 外的接口都必须携带：

```http
Authorization: Bearer <MODELPILOT_API_KEY>
```

## 接口列表

| 方法 | 路径 | 作用 |
| --- | --- | --- |
| GET | `/healthz` | 存活检查，无需认证 |
| POST | `/api/v1/benchmarks` | 提交异步任务，返回 202 和任务 ID |
| GET | `/api/v1/benchmarks` | 列出任务元数据（不含完整结果），按创建时间倒序 |
| GET | `/api/v1/benchmarks/{id}` | 查询状态，完成后包含 `result` |
| POST | `/api/v1/benchmarks/{id}/cancel` | 请求取消，返回 202；已经结束返回 409 |
| POST | `/api/v1/comparisons` | 比较两个已完成任务的结果 |

## 1. 提交测评任务

Mock 示例，无需配置允许的模型主机：

```bash
export API_URL=http://localhost:8080
export API_KEY='replace-with-a-long-random-secret'
curl -sS "$API_URL/api/v1/benchmarks" \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "smoke test",
    "model": {"type": "mock", "name": "echo"},
    "dataset": {
      "name": "demo",
      "samples": [
        {"id": "1", "input": "hello", "expected": "hello"},
        {"id": "2", "input": "world", "expected": "world"}
      ]
    },
    "benchmark": {"concurrency": 2, "timeout_seconds": 30, "limit": 100}
  }'
```

返回示例（同时返回 `Location` 查询地址）：

```json
{
  "id": "benchmark-0123456789abcdef0123456789abcdef",
  "status": "running",
  "created_at": "2026-01-01T00:00:00Z"
}
```

OpenAI Compatible 的 `model` 字段示例：

```json
{
  "type": "openai",
  "name": "gpt-4o-mini",
  "endpoint": "https://api.openai.com/v1/chat/completions",
  "token": "UPSTREAM_MODEL_API_KEY",
  "temperature": 0,
  "max_tokens": 4096
}
```

通用 HTTP 模型使用 `type: "http"`，端点接受 `{"input":"...","metadata":{}}`，返回 `{"output":"...","input_tokens":1,"output_tokens":1}`。

### 校验和默认值

- 数据集直接内嵌提交，不需要服务器本地文件路径。
- 请求体最多 8 MiB，最多 10,000 条样本。
- 样本 `id` 省略时自动按 `sample-1` 等补全，最终 ID 必须唯一。
- `model.type` 默认 `mock`，可选 `mock/http/openai`。
- `concurrency` 默认 1，最大 64。
- `timeout_seconds` 默认 30，最大 300，控制单次模型调用超时。
- `limit` 默认 0（全部），大于 0 时取前 N 条样本。
- `temperature` 范围 0..2，`max_tokens` 默认 4096，最大 32768。
- `openai` 模型必须指定 `name` 和完整 Chat Completions 地址。
- 超过同时运行任务上限返回 429，由调用方稍后重试；当前没有待执行队列。
- 模型请求失败反映在结果的成功率和逐请求错误中，不会使整个任务自动变成 `failed`。

## 2. 查询状态和结果

```bash
TASK_ID=benchmark-0123456789abcdef0123456789abcdef
curl -sS "$API_URL/api/v1/benchmarks/$TASK_ID" \
  -H "Authorization: Bearer $API_KEY"
```

调用方每隔 1–3 秒轮询即可，客户端断开连接不会取消任务。

状态：

- `running`：正在执行。
- `completed`：执行结束，`result` 包含任务参数、逐请求结果、聚合性能指标和 Exact Match 质量指标。
- `canceled`：调用取消接口或服务正常关闭导致取消，可能包含部分结果。
- `timed_out`：超过服务配置的最长任务时间，可能包含部分结果。
- `failed`：测评引擎执行失败。
- `interrupted`：服务重启时发现未结束任务。不会自动重跑，需要重新提交。

结果字段沿用现有 CLI 的 `BenchmarkResult`：`summary` 包含成功率、错误率、平均延迟、P50/P90/P95/P99、吞吐量及 Token 指标；`requests` 包含每条样本的输出、状态码和错误分类。

## 3. 取消任务

```bash
curl -sS -X POST "$API_URL/api/v1/benchmarks/$TASK_ID/cancel" \
  -H "Authorization: Bearer $API_KEY"
```

返回 `{"id":"...","status":"cancel_requested"}`。取消是异步的，之后轮询任务获得最终状态。

## 4. 比较结果

```bash
curl -sS "$API_URL/api/v1/comparisons" \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"baseline":"benchmark-BASELINE_ID","candidate":"benchmark-CANDIDATE_ID"}'
```

可选完整 `thresholds` 对象（单位为百分比）：

```json
{
  "baseline": "benchmark-0123456789abcdef0123456789abcdef",
  "candidate": "benchmark-fedcba9876543210fedcba9876543210",
  "thresholds": {"latency": 5, "throughput": 5, "success": 1, "quality": 1}
}
```

省略 `thresholds` 时使用以上默认值；提供对象时未指定的字段为 0（零容忍）。结果返回 `passed`、逐指标差异和 `regressions`。检测到回归也返回 HTTP 200，由调用方判断 `passed`；仅允许比较 `completed` 任务。

## 错误格式

```json
{"error":"task not found"}
```

- 400：参数或 JSON 不合法。
- 401：缺少或不正确的 API 密钥。
- 404：任务不存在。
- 409：任务状态不支持当前操作。
- 413：请求体过大。
- 429：当前运行任务数已达上限。
- 500：存储或服务内部错误。
- 503：服务正在关闭。

## 安全与当前边界

- 本版本是**单实例文件存储服务**，不能多个实例共享同一数据目录。后续横向扩容需数据库和独立任务队列。
- 上游 token 不写入任务配置；逐请求错误中出现该 token 时会替换为 `[REDACTED]`。输入与模型输出仍会持久化，应限制磁盘访问、做好备份和数据保留管理。
- 端点主机必须在管理员允许列表中，禁用 HTTP 跳转。允许列表是可信管理员配置，不是对不可信 DNS 的完整沙箱：请只配置你信任的模型服务，同时使用容器/网络防火墙限制出口，避免 SSRF 与 DNS 重绑定风险。
- 共享 API 密钥不区分用户，所有持钥调用方都能查看所有任务。公网多租户场景还需身份管理、权限隔离、请求限流和审计。
- 列表暂未分页；结果无自动删除策略。大量历史任务需要定期归档数据目录，避免磁盘和列表响应无限增长。
- 当前没有独立 Web UI、数据集管理、自动调优 API 或任务恢复队列；已提供完整基础测评 API 流程。

## 验证

```bash
gofmt -w internal/api cmd/modelpilot-server
go test -race ./...
go vet ./...
docker compose build
```
