# 智能 OnCall 助手

基于 Go（GoFrame）、CloudWego Eino 与 Milvus 的告警分析、RAG 对话与工具调用示例项目。

项目地址: 

---

## 前置环境配置（系统正常运行所需）

按顺序准备以下环境；缺省项会导致对应功能不可用或进程启动失败。

### 1. 基础运行环境

| 组件 | 版本 / 说明 | 用途 |
|------|-------------|------|
| **Go** | `go.mod` 要求 **Go 1.26+** | 编译与运行后端 `main.go` 及各 `internal/ai/cmd` 子命令 |
| **Docker** 与 **Docker Compose** | 当前稳定版即可 | 在 `manifest/docker` 下启动 Milvus（etcd + MinIO + standalone）、可选 Prometheus 测试栈 |
| **Python 3** | 3.x | 前端目录内通过 `python3 -m http.server` 提供静态页（见 `SuperBizAgentFrontend/package.json`） |

### 2. Milvus 向量库

- 代码中 Milvus 地址写死为 **`localhost:19530`**（见 `utility/client/client.go`），数据库名 **`agent`**、集合名 **`biz`**（见 `utility/common/common.go`）。
- 在项目根目录下执行：

```bash
cd manifest/docker
docker compose up -d
```

- 首次连接时客户端会尝试创建库、集合与索引（逻辑在同文件中）。**Attu** 控制台映射为宿主机 **8000** 端口（`8000:3000`），注意与本机其他服务占口冲突。

### 3. Ollama 与向量模型（RAG / 建索引必需）

检索与建索引统一使用 **Ollama 嵌入**，而非配置里的百炼字段（见 `internal/ai/retriever/retriever.go`、`internal/ai/indexer/indexer.go` 中对 `embedder.OllamaEmbedding` 的调用）。

- 安装并启动 [Ollama](https://ollama.com/download)。
- 拉取与代码中维度一致的模型：索引侧向量维度为 **768**（`internal/ai/indexer/indexer.go`），需使用输出 **768 维** 的嵌入模型；仓库示例配置为 **`nomic-embed-text`**，与 `manifest/config/config.example.yaml` 中 `ollama_embedding_model` 一致。
- 在 `manifest/config/config.yaml` 中配置：

```yaml
ollama_embedding_model:
  model: "nomic-embed-text"        # 若换模型，须同步修改 indexer 中 Dimension 与度量类型等
  base_url: "http://localhost:11434"
```

未启动 Ollama 或模型不匹配时，对话中的 Milvus 检索、知识入库会失败。

### 4. 大语言模型（对话与 AI 运维）

- 对话管线、Planner、Executor 通过 **`github.com/cloudwego/eino-ext/components/model/openai`** 以 **OpenAI 兼容协议** 调用配置中的端点（见 `internal/ai/models/open_ai.go`）。
- `manifest/config/config.example.yaml` 注释指向 **火山引擎方舟**（Volcengine Ark）接口与密钥；需自行开通模型权限，并将 **`ds_think_chat_model`**、**`ds_quick_chat_model`** 的 `api_key`、`base_url`、`model` 填写为实际可用值。
- 复制并编辑配置文件（`config.yaml` 已被 `.gitignore` 忽略，勿提交密钥）：

```bash
cp manifest/config/config.example.yaml manifest/config/config.yaml
# 编辑 manifest/config/config.yaml，填写上述模型与密钥
```

### 5. 腾讯云日志 MCP（AI 运维链路强依赖）

`internal/ai/agent/plan_execute_replan/executor.go` 在创建 Executor 时**会调用** `tools.GetLogMcpTool()`。若 MCP 连接失败，**`/api/ai_ops` 无法正常工作**。

- 按 `internal/ai/tools/query_log.go` 文件头注释：开通 CLS、在腾讯云 MCP 页面申请 **SSE 形式的 MCP URL**，并将代码中的 **`mcp_url`** 替换为你的地址（当前为硬编码占位，非生产环境请必改）。

### 6. Prometheus（告警查询工具）

`internal/ai/tools/query_metrics_alerts.go` 中 Prometheus 基址为 **`http://127.0.0.1:9090`**。使用 `query_prometheus_alerts` 时需本地或远程有可访问的 Prometheus；`manifest/docker/docker-compose.yml` 中已包含 Prometheus 与测试用 `test-server` 示例，可按需启动。

### 7. 可选组件

- **MySQL**：仅在被 ReAct 工具 **`mysql_crud`** 调用时需要；日常对话不依赖数据库服务（`hack/config.yaml` 中 `gfcli` 的 MySQL 链接主要用于 GoFrame 代码生成，非 `main` 运行时必需）。
- **百炼 `doubao_embedding_model`**：已在 `embedder` 包中实现，但当前主流程的检索与索引未使用；若将来改为 DashScope 嵌入，需同时调整 Milvus 集合维度与索引配置。

---

## 项目结构概览

| 路径 | 说明 |
|------|------|
| `main.go` | HTTP 服务入口：挂载 `/api`，端口 **6872** |
| `api/chat/v1` | 路由与请求/响应结构体定义 |
| `internal/controller/chat` | 对话、流式、上传、AI 运维控制器 |
| `internal/ai/agent/chat_pipeline` | RAG + ReAct 对话图（Milvus 检索 + LLM） |
| `internal/ai/agent/knowledge_index_pipeline` | Markdown 文件 → 切块 → 向量 → Milvus |
| `internal/ai/agent/plan_execute_replan` | Plan–Execute–Replanner（告警与文档、日志工具） |
| `internal/ai/tools` | Prometheus、内部文档、时间、MySQL、MCP 日志等工具 |
| `manifest/config` | 运行时配置目录（使用 `config.yaml`） |
| `manifest/docker` | Milvus / Prometheus 等 Compose |
| `SuperBizAgentFrontend` | 静态前端（`app.js` 中 `apiBaseUrl` 指向 `http://localhost:6872/api`） |
| `docs/` | 知识库 Markdown 示例；`file_dir` 配置指向的上传目录用于附件等 |

---

## 配置说明

运行时从 **GoFrame 默认配置路径** 读取（与仓库惯例一致，将 `config.yaml` 放在 `manifest/config/`）。`main.go` 会读取 **`file_dir`**，用于文件上传等逻辑，请指向存在且可写的目录（示例为 `./docs`）。

关键字段与 `manifest/config/config.example.yaml` 保持一致即可，至少包括：

- `ds_think_chat_model` / `ds_quick_chat_model`：Planner 与对话、执行所用 LLM。
- `doubao_embedding_model`：预留；当前主路径未用。
- `ollama_embedding_model`：**当前 RAG 与索引用**。
- `file_dir`：上传与知识文件相关根目录。

---

## 启动步骤

### 1. 启动 Milvus（及可选监控栈）

```bash
cd manifest/docker
docker compose up -d
```

### 2. 准备配置与 Ollama

```bash
cp manifest/config/config.example.yaml manifest/config/config.yaml
# 编辑 config.yaml；启动 Ollama 并拉取嵌入模型
```

### 3. 构建知识索引（可选但 RAG 建议执行）

在项目**根目录**执行（`knowledge_cmd` 内对 `./docs` 的遍历相对于当前工作目录）：

```bash
go run ./internal/ai/cmd/knowledge_cmd
```

将 `docs` 下 Markdown 写入 Milvus 集合 `biz`（嵌入与维度与 `indexer` 实现一致）。

### 4. 启动后端

```bash
go run main.go
```

服务监听 **`http://127.0.0.1:6872`**，API 前缀 **`/api`**。

### 5. 启动前端

```bash
cd SuperBizAgentFrontend
npm run start
# 或: python3 -m http.server 8080
```

浏览器访问脚本提示的地址（默认 **8080**）。若后端部署在其他主机或端口，需修改 `SuperBizAgentFrontend/app.js` 中的 **`apiBaseUrl`**。

---

## HTTP API

定义见 `api/chat/v1/chat.go`，实际路径在 `/api` 分组下：

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/chat` | 非流式对话 |
| POST | `/api/chat_stream` | 流式对话（SSE，由控制器与 GoFrame 协同推送） |
| POST | `/api/upload` | 文件上传（multipart） |
| POST | `/api/ai_ops` | AI 运维分析（Plan–Execute–Replanner + MCP 日志等工具） |

请求体字段：`ChatReq` / `ChatStreamReq` 含会话 **`Id`** 与 **`Question`**；会话历史由 `utility/mem` 中按 ID 维护的滑动窗口保存。

---

## 核心逻辑摘要

1. **对话（`/api/chat`、`/api/chat_stream`）**  
   构建 `UserMessage` → `chat_pipeline` 编排图：并行 **`InputToRag` → MilvusRetriever（TopK=3，Ollama 嵌入）** 与 **`InputToChat`** → `ChatTemplate` → **ReAct**（`OpenAIForDeepSeekV3Quick`），工具含 Prometheus 告警、MySQL CRUD、内部文档检索等（见 `internal/ai/agent/chat_pipeline/reAgent.go`）。

2. **知识入库**  
   文件 Loader → Markdown 切分 → **Ollama 嵌入** → Milvus2 Indexer，向量维度 **768**。

3. **AI 运维（`/api/ai_ops`）**  
   **Planner**：`OpenAIForDeepSeekV31Think`；**Executor**：`OpenAIForDeepSeekV3Quick` + **MCP 日志工具** + Prometheus + 内部文档等；Replanner 迭代直至流程结束（迭代上限在代码中配置为极大值，实际由模型与工具结果决定）。

---

## 其他命令与测试

仓库内还有例如 `internal/ai/cmd/recall_cmd`、`internal/ai/cmd/llm_tool_cmd` 等入口，用于召回测试或模型调试，可按需 `go run` 指定包路径。

---

## 文档与运维手册

- 告警与运维说明可参考仓库 `docs/` 下 Markdown（如 `docs/告警处理手册.md`）。

若你后续将 MCP URL、Prometheus 地址或嵌入提供方改为可配置项，建议同步更新本节与 `manifest/config/config.example.yaml` 中的注释，以便部署一致。
