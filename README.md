# ModelPilot

GitHub: [ayu-v0/ModelPilot](https://github.com/ayu-v0/ModelPilot)

ModelPilot 是一个基于 Go 构建的模型性能测评与自动调优平台，面向大语言模型、机器学习模型以及模型推理服务，提供统一的测评、分析、实验编排和自动优化能力。

项目目标是将“模型性能评估”和“模型性能调优”整合为一套可复现、可扩展、可自动化的工程机制，帮助团队持续回答以下问题：

- 当前模型在准确性、延迟、吞吐量和资源消耗方面表现如何？
- 不同模型、版本、推理参数和运行环境之间有什么差异？
- 在满足质量约束的前提下，怎样自动找到更优的配置？
- 每次调优是否可复现、可追踪、可比较，并能安全发布？

## 核心功能

### 1. 模型性能测评

- 支持离线数据集和在线请求流量两类测评方式
- 支持多模型、多版本、多运行环境对比
- 支持同步和异步推理任务
- 支持固定测试集、随机采样和自定义数据集
- 支持基准测评、回归测评和压力测评
- 支持测评任务暂停、恢复、取消和重试

### 2. 多维度指标采集

#### 质量指标

- 分类、回归、排序等任务指标
- 生成式任务的规则指标、评审模型指标和人工评估指标
- 准确率、精确率、召回率、F1、BLEU、ROUGE 等可扩展指标
- 安全性、事实性、相关性和一致性指标

#### 性能指标

- 首 Token 延迟（TTFT）
- 单 Token 延迟和端到端延迟
- 吞吐量、并发数和 QPS
- 成功率、超时率、错误率
- 输入/输出 Token 数和 Token 吞吐

#### 资源与成本指标

- CPU、内存、GPU、显存利用率
- 功耗和运行时资源消耗
- 单次请求成本和单位 Token 成本
- 模型部署实例规模与资源利用率

### 3. 自动调优

- 支持推理参数调优，例如 batch size、并发度、超时、采样参数等
- 支持模型量化、精度、编译和运行时配置调优
- 支持搜索空间、约束条件和优化目标定义
- 支持网格搜索、随机搜索、贝叶斯优化等策略
- 支持单目标和多目标优化
- 支持质量门禁，避免性能提升导致模型质量下降
- 支持基于历史实验结果的增量优化
- 支持自动生成候选配置、执行实验、评估结果和选择最优方案

### 4. 实验与结果管理

- 统一记录模型、数据集、代码版本、运行环境和配置
- 保存每次实验的原始结果、聚合指标和日志
- 支持实验结果对比、排序和筛选
- 支持基线管理和性能回归检测
- 支持实验复现和结果导出

### 5. 可扩展架构

- 通过模型适配器接入不同模型和推理服务
- 通过指标插件扩展自定义评测逻辑
- 通过执行器适配本地、容器、远程服务和 GPU 集群
- 通过存储接口接入文件、数据库和对象存储
- 通过优化器接口扩展新的自动调优算法

## 总体架构

```text
                 +----------------------+
                 |      API / CLI        |
                 +----------+-----------+
                            |
                 +----------v-----------+
                 |   Orchestrator        |
                 | 测评与调优任务编排     |
                 +----+-------------+----+
                      |             |
          +-----------v--+     +----v-----------+
          |   Benchmark   |     |   Optimizer    |
          | 数据集与指标   |     | 搜索与决策      |
          +-----------+----+     +----+-----------+
                      |             |
                 +----v-------------v----+
                 |       Executor        |
                 | 模型调用与资源采集      |
                 +----+-------------+----+
                      |             |
          +-----------v--+     +----v-----------+
          | Model Adapter |     | Metric Plugin  |
          | 模型/服务接入  |     | 指标计算扩展     |
          +---------------+     +----------------+
                            |
                 +----------v-----------+
                 |       Storage        |
                 | 实验、指标、日志、结果 |
                 +----------------------+
```

## 项目目录

```text
.
├── cmd/
│   └── modelpilot/          # 应用程序入口
├── internal/
│   ├── api/                 # HTTP/RPC API 及请求处理
│   ├── benchmark/           # 测评任务、数据集和指标编排
│   ├── config/              # 配置加载与校验
│   ├── domain/              # 核心领域模型和接口
│   ├── executor/            # 模型调用、任务执行和资源采集
│   ├── optimizer/           # 自动调优策略与搜索算法
│   ├── orchestrator/        # 测评/调优流程编排
│   └── storage/             # 实验结果和元数据存储
├── pkg/
│   ├── metrics/             # 可复用指标定义
│   ├── model/               # 模型接入公共接口
│   └── plugin/              # 插件扩展接口
├── configs/                 # 配置文件示例
├── deployments/             # Docker、Kubernetes 等部署资源
├── docs/                    # 设计文档和 API 文档
├── examples/                # 使用示例
├── scripts/                 # 开发和自动化脚本
├── test/
│   ├── integration/         # 集成测试
│   └── testdata/            # 测试数据
├── go.mod
└── README.md
```

## 典型工作流

1. 注册模型或推理服务，并配置访问地址、认证信息和运行参数。
2. 创建测评任务，选择数据集、指标、并发策略和资源采集项。
3. 执行基准测评，生成模型质量、性能、资源和成本报告。
4. 创建调优任务，设置可调参数、搜索空间、优化目标和质量约束。
5. 系统自动生成候选配置并执行多轮实验。
6. 对实验结果进行评估、排序和质量门禁校验。
7. 选择最优配置，导出报告或发布到目标环境。
8. 持续执行回归测评，发现模型或服务性能退化。

## 快速开始

> 已提供 CLI 和可独立部署的 HTTP API 服务。部署后可通过接口提交异步测评、查询状态与结果、取消任务及比较基线，无需登录服务器执行 CLI。

### Web API 部署

创建 `.env` 并设置服务访问密钥：

```dotenv
MODELPILOT_API_KEY=replace-with-a-long-random-secret
MODELPILOT_ALLOWED_HOSTS=api.openai.com,inference.example.com
```

```bash
docker compose up -d --build
curl http://localhost:8080/healthz
```

也可以直接运行 `go run ./cmd/modelpilot-server`（必须设置 `MODELPILOT_API_KEY`）。默认监听 `:8080`，Compose 仅绑定本机端口，远程访问建议通过 HTTPS 反向代理。

**完整部署说明、接口参数和 curl 示例：[HTTP API 文档](docs/api.md)**。

| 方法 | 接口 | 功能 |
| --- | --- | --- |
| POST | `/api/v1/benchmarks` | 提交内嵌 JSON 数据集，异步测评 |
| GET | `/api/v1/benchmarks` | 任务列表 |
| GET | `/api/v1/benchmarks/{id}` | 查询任务和完整结果 |
| POST | `/api/v1/benchmarks/{id}/cancel` | 取消测评 |
| POST | `/api/v1/comparisons` | 基线对比 |

所有业务接口通过 `Authorization: Bearer <MODELPILOT_API_KEY>` 认证。结果持久化保存，服务重启时未完成任务标记为 `interrupted`；当前适用于单实例部署。

### 环境要求

- Go 1.22 或更高版本
- 可选：Docker、Kubernetes、GPU 运行环境

### 初始化与运行

当前已实现第一版离线模型性能测评链路，支持 JSONL 数据集、Mock/HTTP/OpenAI Compatible 模型、并发执行、延迟分布、吞吐量、错误率、Token 吞吐和 Exact Match 质量指标。结果以 JSON 持久化，便于后续基线比较和自动调优。

```bash
go mod tidy
go run ./cmd/modelpilot -dataset ./examples/dataset.jsonl -concurrency 4
```

也可以使用配置文件运行：

```bash
go run ./cmd/modelpilot -config ./configs/benchmark.example.yaml
```

结果默认保存到 `./data/results`，也可以通过 `-output` 指定目录。命令输出中的 `task` 是结果文件名对应的任务 ID。

### 基线比较与回归检测

可以比较两个已经保存的测评结果：

```bash
go run ./cmd/modelpilot \
  -baseline benchmark-123 \
  -candidate benchmark-456 \
  -output ./data/results
```

也可以将比较参数写入配置文件的 `compare` 字段，然后执行：

```bash
go run ./cmd/modelpilot -config ./configs/benchmark.example.yaml
```

比较默认检查以下指标：

- 平均延迟、P50、P95、P99 延迟：候选结果变慢超过 5% 判定回归
- 成功率：下降超过 1% 判定回归
- 吞吐量：下降超过 5% 判定回归
- 质量指标：下降超过 1% 判定回归

可以按需调整阈值：

```bash
go run ./cmd/modelpilot \
  -baseline benchmark-123 \
  -candidate benchmark-456 \
  -latency-threshold 10 \
  -throughput-threshold 8 \
  -success-threshold 0.5 \
  -quality-threshold 2
```

比较通过时进程退出码为 `0`，检测到回归时退出码为 `2`，加载结果失败时退出码为 `1`。使用 `-json` 可以输出机器可读的比较报告。

### 限制测评数据量

使用 `-limit` 可以限制本次测评最多处理的数据集样本数量，程序按 JSONL 文件顺序取前 N 条。`0` 或不配置表示处理全部数据。

```bash
./dist/modelpilot-linux-amd64 \
  -dataset ./examples/dataset.jsonl \
  -model-type mock \
  -limit 100 \
  -concurrency 4
```

配置文件方式：

```yaml
benchmark:
  concurrency: 4
  timeout_seconds: 30
  limit: 100
```

使用通用 HTTP JSON 模型服务：

```bash
go run ./cmd/modelpilot \
  -dataset ./examples/dataset.jsonl \
  -model-type http \
  -model-name my-http-model \
  -endpoint http://localhost:8080/invoke \
  -concurrency 4
```

HTTP 模型服务请求格式：

```json
{
  "input": "hello",
  "metadata": {}
}
```

HTTP 模型服务响应格式：

```json
{
  "output": "hello",
  "input_tokens": 1,
  "output_tokens": 1
}
```

使用 OpenAI Compatible Chat Completions 服务：

```bash
go run ./cmd/modelpilot \
  -dataset ./examples/dataset.jsonl \
  -model-type openai \
  -model-name gpt-4o-mini \
  -endpoint http://localhost:8000/v1/chat/completions \
  -token YOUR_API_KEY \
  -concurrency 4 \
  -max-tokens 512
```

### 测试与检查

```bash
go test ./...
go vet ./...
gofmt -w .
```

## 设计原则

- **可复现**：完整记录模型、数据、配置、代码和运行环境。
- **可比较**：统一指标定义、测评协议和结果格式。
- **可扩展**：模型、指标、执行器、存储和优化策略均通过接口扩展。
- **质量优先**：自动调优必须满足质量阈值和安全约束。
- **资源感知**：同时关注效果、延迟、吞吐、资源和成本。
- **渐进式落地**：先支持本地和 HTTP 推理服务，再扩展到容器和集群环境。

## 当前开发进度

- [x] 核心领域模型和模型调用接口
- [x] Mock 模型适配器
- [x] HTTP 模型适配器
- [x] OpenAI Compatible Chat Completions 模型适配器
- [x] JSONL 数据集读取
- [x] Worker Pool 并发测评
- [x] 成功率、错误率、最小/平均/最大延迟
- [x] P50/P90/P95/P99 延迟分位数
- [x] 请求吞吐量、Token 吞吐量和平均 Token 统计
- [x] 输入/输出 Token 总量统计
- [x] Exact Match 质量指标
- [x] JSON 文件结果持久化
- [x] 基础命令行测评入口
- [x] 基线结果比较与性能回归检测
- [x] YAML/JSON 配置文件加载
- [x] 数据集样本数量限制

## 规划中的能力

- [x] HTTP/OpenAI Compatible 模型适配器接入命令行
- [x] 异步测评任务 API（内嵌数据集提交、状态、结果、取消）
- [ ] 独立数据集管理 API
- [ ] 更多指标插件
- [x] 基准测试与回归检测
- [ ] 网格搜索和随机搜索调优器
- [ ] 多目标自动调优
- [ ] Prometheus 指标与 Grafana 看板
- [ ] Docker/Kubernetes 执行器
- [x] HTTP API、Bearer 认证与 Docker Compose 部署
- [ ] 独立 Web UI

## 发布与二进制构建

项目仓库地址：

```text
https://github.com/ayu-v0/ModelPilot
```

Windows PowerShell 下构建多平台二进制：

```powershell
.\scripts\build.ps1 -Version v0.1.0
```

构建产物位于 `dist/`：

```text
dist/modelpilot-windows-amd64.exe
dist/modelpilot-linux-amd64
dist/modelpilot-darwin-amd64
dist/modelpilot-darwin-arm64
```

GitHub Actions 工作流位于 `.github/workflows/build.yml`。推送代码后会自动运行测试并构建多平台产物。

## 许可证

本项目许可证见 [LICENSE](./LICENSE)。
