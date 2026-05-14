# 智能Oncall助手系统架构与实现流程分析

## 一、系统概览

**系统名称**：SuperBizAgent（智能OnCall助手）  
**核心功能**：基于AI的告警处理、智能问答与运维分析  
**技术栈**：  
- 后端：Go + GoFrame + Eino AI框架
- 前端：原生JavaScript + HTML/CSS
- AI服务：Qwen3向量模型（配置中的doubao_embedding_model，其实是千问3）/Ollama本地向量模型/DeepSeek模型
- 向量数据库：`Milvus` （需要去 `manifest/docker`目录下，执行 `docker-compose up -d` 将 `Milvus` 向量数据库启动起来）
- 日志系统：腾讯云CLS（通过MCP协议）

---

## 二、后端系统架构

### 2.1 整体分层架构

```
┌─────────────────────────────────────────────┐
│        前端 (HTML/JS)                       │
└────────────┬────────────────────────────────┘
             │ HTTP/SSE
             ↓
┌─────────────────────────────────────────────┐
│ 路由层 (api/chat/v1)                        │
├─────────────────────────────────────────────┤
│ Chat | ChatStream | FileUpload | AIOps     │
└────────────┬────────────────────────────────┘
             │
┌─────────────────────────────────────────────┐
│ 控制层 (internal/controller/chat)           │
├─────────────────────────────────────────────┤
│ ControllerV1: Chat() ChatStream() AIOps()   │
└────────────┬────────────────────────────────┘
             │
┌─────────────────────────────────────────────┐
│ 业务逻辑层 (internal/logic/chat)            │
├─────────────────────────────────────────────┤
│ ChatService | SSEService                    │
└────────────┬────────────────────────────────┘
             │
┌─────────────────────────────────────────────┐
│ AI管道层 (internal/ai/agent)                │
├─────────────────────────────────────────────┤
│ • ChatPipeline（RAG+ReAct对话）             │
│ • KnowledgeIndexPipeline（知识索引）        │
│ • PlanExecuteReplan（计划-执行-重规划）     │
└────────────┬────────────────────────────────┘
             │
┌─────────────────────────────────────────────┐
│ 工具/组件层                                 │
├─────────────────────────────────────────────┤
│ • Tools: 告警查询 | 日志查询 | 文档查询等   │
│ • Models: OpenAI/DeepSeek模型集成          │
│ • Retriever: Milvus向量检索                │
│ • Embedder: 向量编码                        │
└────────────┬────────────────────────────────┘
             │
┌─────────────────────────────────────────────┐
│ 基础设施层                                  │
├─────────────────────────────────────────────┤
│ • Memory: 会话内存管理                      │
│ • Client: Milvus/MySQL连接                 │
│ • Middleware: CORS/Response处理             │
└─────────────────────────────────────────────┘
```

### 2.2 主入口点

**文件**：[main.go](main.go)

```go
func main() {
    ctx := gctx.New()
    s := g.Server()  // GoFrame服务器实例
    
    s.Group("/api", func(group *ghttp.RouterGroup) {
        group.Middleware(middleware.CORSMiddleware)      // CORS跨域
        group.Middleware(middleware.ResponseMiddleware)  // 响应格式统一
        group.Bind(chat.NewV1())  // 绑定Chat控制器
    })
    
    s.SetPort(6872)  // 监听端口
    s.Run()
}
```

**启动流程**：
1. 初始化GoFrame框架上下文
2. 设置中间件（CORS、响应格式化）
3. 注册API路由组 `/api`
4. 启动HTTP服务器（端口6872）

---

### 2.3 API端点与请求类型

**文件**：[api/chat/v1/chat.go](api/chat/v1/chat.go)

| 端点 | 方法 | 功能 | 请求类型 |
|------|------|------|---------|
| `/api/chat` | POST | 快速对话 | ChatReq → ChatRes |
| `/api/chat_stream` | POST | 流式对话 | ChatStreamReq → ChatStreamRes |
| `/api/upload` | POST | 文件上传 | FileUploadReq → FileUploadRes |
| `/api/ai_ops` | POST | AI运维分析 | AIOpsReq → AIOpsRes |

**核心请求结构**：
```go
type ChatReq struct {
    Id       string  // 会话ID
    Question string  // 用户问题
}

type ChatStreamReq struct {
    Id       string  // 会话ID
    Question string  // 用户问题
}

type AIOpsReq struct {
    // AI运维分析的统一请求入口
}
```

---

### 2.4 控制层实现

**文件**：[internal/controller/chat/](internal/controller/chat/)

#### 2.4.1 Chat - 快速对话接口

**文件**：[chat_v1_chat.go](internal/controller/chat/chat_v1_chat.go)

```
请求流程：
  ChatReq(Id, Question)
    ↓
  构建UserMessage {ID, Query, History}
    ↓
  调用chat_pipeline.BuildChatAgent(ctx)构建对话agent
    ↓
  agent.Invoke() 执行推理
    ↓
  将用户消息和AI回复存入会话内存
    ↓
  返回 ChatRes(Answer)
```

**关键代码逻辑**：
```go
func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error) {
    userMessage := &chat_pipeline.UserMessage{
        ID:      req.Id,
        Query:   req.Question,
        History: mem.GetSimpleMemory(req.Id).GetMessages(),  // 获取历史对话
    }
    
    runner, err := chat_pipeline.BuildChatAgent(ctx)  // 构建聊天管道
    out, err := runner.Invoke(ctx, userMessage)       // 执行推理
    
    // 更新会话内存
    mem.GetSimpleMemory(req.Id).SetMessages(schema.UserMessage(req.Question))
    mem.GetSimpleMemory(req.Id).SetMessages(schema.SystemMessage(out.Content))
    
    return &v1.ChatRes{Answer: out.Content}, nil
}
```

#### 2.4.2 ChatStream - 流式对话接口

**文件**：[chat_v1_chat_stream.go](internal/controller/chat/chat_v1_chat_stream.go)

```
请求流程：
  ChatStreamReq(Id, Question)
    ↓
  创建SSE(Server-Sent Events)客户端连接
    ↓
  构建UserMessage {ID, Query, History}
    ↓
  调用agent.Stream()获取流式响应
    ↓
  逐块接收chunk，发送给客户端 (SendToClient)
    ↓
  流结束时保存完整对话到内存
    ↓
  返回完成信号
```

**数据流特点**：
- 使用WebSocket或SSE实现服务器推送
- 实时流式返回AI的逐块生成结果
- 在流式传输中维持客户端连接

#### 2.4.3 AIOps - AI运维分析接口

**文件**：[chat_v1_ai_ops.go](internal/controller/chat/chat_v1_ai_ops.go)

```
执行流程：
  AIOpsReq
    ↓
  调用plan_execute_replan.BuildPlanAgent(ctx, query)
    ↓
  [规划阶段 Planner]
    • 使用DeepSeek V3思维模型生成处理计划
    
  [执行阶段 Executor]
    • 调用工具集：
      - query_prometheus_alerts: 查询活跃告警
      - query_internal_docs: 查询告警处理文档
      - get_current_time: 获取当前时间
      - query_log: 查询日志（通过MCP）
    
  [重规划阶段 Replanner]
    • 根据执行结果重新评估计划
    • 迭代最多20次
    
  返回 AIOpsRes(Result, Detail)
    Result: 最终的告警分析报告
    Detail: 各步骤的详细输出
```

**AI运维核心Prompt**：
```
1. 获取所有活跃告警 → query_prometheus_alerts
2. 逐个查询告警对应的处理文档 → query_internal_docs
3. 查询相关日志和指标 → query_log/query_metrics
4. 生成告警分析报告格式：
   - 告警处理详情
   - 活跃告警清单
   - 告警根因分析
   - 处理方案执行
   - 结论
```

---

### 2.5 AI管道系统（核心智能引擎）

#### 2.5.1 Chat Pipeline - 对话管道

**文件**：[internal/ai/agent/chat_pipeline/orchestration.go](internal/ai/agent/chat_pipeline/orchestration.go)

**架构图**：
```
                    START
                      ↓
         ┌────────────┴────────────┐
         ↓                         ↓
   InputToRag                InputToChat
   (Query提取)              (上下文构建)
         ↓                         ↓
         ↓          ┌──────────────┘
         ↓          ↓
   MilvusRetriever  [获取向量数据库中的相关文档]
   (RAG检索)
         ↓
         └──────────────┐
                        ↓
                  ChatTemplate
                  (提示词模板)
                  含Context信息：
                  • System Prompt
                  • 对话历史
                  • 用户Query
                  • 检索到的文档
                        ↓
                  ReactAgent
                  (LLM推理+工具调用)
                  • 使用DeepSeek V3 Quick
                  • 支持函数调用
                        ↓
                       END
```

**管道流程**：

| 步骤 | 节点 | 功能 | 输入 | 输出 |
|------|------|------|------|------|
| 1 | InputToRag | 提取查询词 | UserMessage | Query字符串 |
| 2 | InputToChat | 构建上下文 | UserMessage | {content, history, date} |
| 3 | MilvusRetriever | 向量检索 | Query | 相关文档列表 |
| 4 | ChatTemplate | 组织提示词 | docs + history + query | 格式化消息 |
| 5 | ReactAgent | LLM推理 | 格式化消息 | 最终回复 |

**关键代码**：
```go
func BuildChatAgent(ctx context.Context) compose.Runnable[*UserMessage, *schema.Message] {
    g := compose.NewGraph[*UserMessage, *schema.Message]()
    
    // 添加节点
    g.AddLambdaNode("InputToRag", newInputToRagLambda)
    g.AddChatTemplateNode("ChatTemplate", chatTemplate)
    g.AddLambdaNode("ReactAgent", reactAgent)
    g.AddRetrieverNode("MilvusRetriever", milvusRetriever, compose.WithOutputKey("documents"))
    g.AddLambdaNode("InputToChat", newInputToChatLambda)
    
    // 定义边（数据流向）
    g.AddEdge(compose.START, "InputToRag")
    g.AddEdge(compose.START, "InputToChat")
    g.AddEdge("InputToRag", "MilvusRetriever")
    g.AddEdge("MilvusRetriever", "ChatTemplate")
    g.AddEdge("InputToChat", "ChatTemplate")
    g.AddEdge("ChatTemplate", "ReactAgent")
    g.AddEdge("ReactAgent", compose.END)
    
    return g.Compile(ctx)
}
```

**系统提示词（关键）**：

```yaml
角色: 对话小助手
核心能力:
  - 上下文理解与对话
  - 网络搜索获取信息
  - 调用工具辅助
互动指南:
  - 完全理解用户需求
  - 清晰简洁地回复
  - 参考相关文档
输出要求:
  - 易读、结构良好
  - 纯文本格式（无Markdown）
  - 包含当前日期和相关文档内容
```

---

#### 2.5.2 Knowledge Index Pipeline - 知识索引管道

**文件**：[internal/ai/agent/knowledge_index_pipeline/orchestration.go](internal/ai/agent/knowledge_index_pipeline/orchestration.go)

**用途**：处理知识库构建，从文件到向量数据库

```
FILE INPUT (Markdown文件)
    ↓
[FileLoader] - 加载文件内容
    ↓
[MarkdownSplitter] - 按Markdown结构分割
    ↓
[Embedding] - 转换为向量
    ↓
[MilvusIndexer] - 存入向量数据库
    ↓
OUTPUT: 索引IDs列表
```

**数据处理流程**：
1. **加载**：从文件系统读取Markdown文档
2. **分割**：按Markdown标题、段落分割成块
3. **向量化**：使用Embedder（豆宝Embedding）将文本转为向量
4. **索引**：将向量和元数据存入Milvus

---

#### 2.5.3 Plan-Execute-Replan Pipeline - 规划-执行-重规划

**文件**：[internal/ai/agent/plan_execute_replan/plan_execute_replan.go](internal/ai/agent/plan_execute_replan/plan_execute_replan.go)

**架构**：

```
Query (用户的复杂请求)
  ↓
[Planner - DeepSeek V3思维模型]
  • 深度思考问题
  • 生成详细执行计划
  • 输出：Plan
  ↓
[Executor - DeepSeek V3 Quick模型]
  • 执行计划步骤
  • 调用可用工具
  • 获取中间结果
  ↓
[Replanner - 评估与重规划]
  • 检查执行结果
  • 决定是否继续、修改或结束
  • 支持最多20次迭代
  ↓
Final Output (最终分析报告)
```

**执行流程详解**：

```go
func BuildPlanAgent(ctx, query string) (string, []string, error) {
    planner := NewPlanner(ctx)        // DeepSeek V3思维
    executor := NewExecutor(ctx)      // 执行者+工具集
    replanner := NewRePlanAgent(ctx)  // 重规划器
    
    // 创建Plan-Execute-Replan流程
    planExecuteAgent := planexecute.New(ctx, &planexecute.Config{
        Planner:       planner,
        Executor:      executor,
        Replanner:     replanner,
        MaxIterations: 20,  // 最多20轮迭代
    })
    
    // 执行，收集所有中间步骤
    result = runner.Query(ctx, query)
    
    return finalMessage, detailSteps, nil
}
```

**可用工具集** (Executor中调用)：
- `query_prometheus_alerts`: 查询Prometheus告警
- `query_internal_docs`: 查询内部文档(RAG)
- `get_current_time`: 获取当前时间
- `query_log`: 查询日志 (通过MCP)
- `query_metrics_alerts`: 查询指标和告警

---

### 2.6 工具系统

**位置**：[internal/ai/tools/](internal/ai/tools/)

#### 2.6.1 Query Internal Docs Tool - 内部文档查询

```go
func NewQueryInternalDocsTool() tool.InvokableTool {
    // 用途：搜索内部知识库获取处理步骤
    // 实现：基于Milvus的RAG检索
    // 输入：{query: string}
    // 输出：JSON格式的相关文档
}
```

**流程**：
1. 用户查询 → Embedding向量化
2. Milvus向量相似度检索
3. 返回Top-K相关文档

#### 2.6.2 Query Metrics Alerts Tool - 告警查询

```go
type PrometheusAlert struct {
    Labels       map[string]string  // 告警标签
    Annotations  map[string]string  // 注解信息
    State        string             // firing/pending
    ActiveAt     string             // RFC3339时间戳
}

func queryPrometheusAlerts() ([]SimplifiedAlert, error) {
    // 查询Prometheus /api/v1/alerts端点
    // 解析并简化告警信息
    // 计算告警持续时间
}
```

**返回格式**：
```json
{
    "success": true,
    "alerts": [
        {
            "alert_name": "HighCPU",
            "description": "CPU使用率过高",
            "state": "firing",
            "active_at": "2025-10-29T08:48:42Z",
            "duration": "2h30m15s"
        }
    ]
}
```

#### 2.6.3 MySQL CRUD Tool - 数据库操作

```go
type MysqlCrudInput struct {
    DSN         string  // 数据库连接字符串
    SQL         string  // SQL语句
    OperateType string  // query/insert/update/delete
}
```

**特点**：
- 支持任意SQL操作
- 执行前需用户确认
- 结果JSON格式返回

#### 2.6.4 Log MCP Tool - 日志查询

**实现方式**：通过Model Context Protocol(MCP)连接腾讯云日志服务

```go
func GetLogMcpTool() ([]tool.BaseTool, error) {
    // 连接MCP服务器
    cli := client.NewSSEMCPClient(mcp_url)
    
    // 获取可用的日志查询工具
    mcpTools := e_mcp.GetTools(ctx, &e_mcp.Config{Cli: cli})
    
    return mcpTools, nil
}
```

**配置项** (系统提示词中)：
- 日志主题地域：`ap-guangzhou`
- 日志主题ID：`869830db-a055-4479-963b-3c898d27e755`

---

### 2.7 向量检索与Embedding

**Retriever** - [internal/ai/retriever/retriever.go](internal/ai/retriever/retriever.go)

```go
func NewMilvusRetriever(ctx context.Context) retriever.Retriever {
    cli := client.NewMilvusClient(ctx)     // Milvus连接
    eb := embedder.OllamaEmbedding(ctx)    // 豆宝向量编码器
    
    // 创建检索器配置
    config := &milvus.RetrieverConfig{
        Client:      cli,
        Collection: "知识库集合名",
        VectorField: "vector",
        OutputFields: []string{"id", "content", "metadata"},
        TopK: 1,  // 返回最相关的1条文档
        Embedding: eb,
    }
    
    return milvus.NewRetriever(ctx, config)
}
```

**流程**：
```
用户Query
  ↓
Embedding(豆宝模型)
  ↓
生成Query向量
  ↓
Milvus相似度检索(TopK=1)
  ↓
返回最相关文档
```

---

### 2.8 会话内存管理

**文件**：[utility/mem/mem.go](utility/mem/mem.go)

**设计**：基于会话ID的内存缓存

```go
type SimpleMemory struct {
    ID            string            // 会话ID
    Messages      []*schema.Message // 消息历史
    MaxWindowSize int               // 最大窗口 = 6条消息
}

// 消息管理
SetMessages(msg)  // 添加消息，超出窗口大小时删除最早的消息对
GetMessages()     // 获取当前窗口内的消息
```

**重要特性**：
- 维持对话历史，每个会话独立
- 滑动窗口大小：6条消息（3轮对话）
- 删除消息时保持用户-AI的成对关系
- 线程安全（使用sync.Mutex）

---

### 2.9 LLM模型集成

**文件**：[internal/ai/models/open_ai.go](internal/ai/models/open_ai.go)

**支持两个模型版本**：

| 模型 | 用途 | 特点 | 配置来源 |
|------|------|------|---------|
| DeepSeek V3.1 思维 | Plan-Execute-Replan的Planner | 深度思维推理，处理复杂问题 | `ds_think_chat_model` |
| DeepSeek V3 Quick | Chat Pipeline、Executor | 快速响应，成本优化 | `ds_quick_chat_model` |

**配置方式**（从config.yaml）：
```yaml
ds_think_chat_model:
  model: "deepseek-reasoner"
  api_key: "YOUR_API_KEY"
  base_url: "https://api.deepseek.com"

ds_quick_chat_model:
  model: "deepseek-chat"
  api_key: "YOUR_API_KEY"
  base_url: "https://api.deepseek.com"
```

### 3.0 向量模型

配置文件中有两个向量模型：
- 阿里的百炼模型，你自己需要登录 https://bailian.console.aliyun.com/cn-beijing?tab=home#/home 官网，去获取 `api key` 并且还需要把 向量模型 `text-embedding-v4`使用权限开通了即可。

- 当前也可以选择安装 `Ollama` 使用本地部署的向量模型（我这里用的这种方式）
  ```bash
    # Ollama的下载地址
    https://ollama.com/download

    # 如果觉得 Embedding的模型不够好，这里还有更多的模型，记得修改配置中的模型名称即可
    https://ollama.com/search?c=embedding

    # 启动 Ollama(直接点击你安装好的程序，来启动) 并下载 nomic-embed-text 模型（这里下载好会自动运行这个模型的）
    ollama pull nomic-embed-text

  ```

**配置方式**（从config.yaml）：

```yaml
doubao_embedding_model:
  api_key: "这里使用阿里百炼的api"
  base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
  model: "text-embedding-v4"

ollama_embedding_model:
  model: "nomic-embed-text"
  base_url: "http://localhost:11434"
```
---

## 三、前端系统详解

### 3.1 前端架构

**文件**：[SuperBizAgentFrontend/](SuperBizAgentFrontend/)

```
HTML结构 (index.html)
  ├─ 侧边栏 (Sidebar)
  │  ├─ 新建对话按钮
  │  └─ 近期对话历史列表
  ├─ 主内容区 (Main Content)
  │  ├─ AI Ops按钮
  │  ├─ 欢迎信息
  │  ├─ 消息展示区
  │  └─ 聊天输入区
  │     ├─ 文本输入框
  │     ├─ 文件上传工具
  │     ├─ 对话模式选择器
  │     └─ 发送按钮
  └─ 加载遮罩层 (Loading Overlay)

应用逻辑 (app.js)
  ├─ 初始化 (constructor)
  ├─ DOM元素管理
  ├─ 事件绑定
  ├─ API通信
  ├─ UI渲染
  └─ 数据持久化 (LocalStorage)
```

### 3.2 前端初始化流程

**主类**：`SuperBizAgentApp`

```javascript
constructor() {
    this.apiBaseUrl = 'http://localhost:6872/api';
    this.currentMode = 'quick';  // 对话模式
    this.sessionId = this.generateSessionId();  // 生成会话ID
    this.isStreaming = false;
    this.currentChatHistory = [];  // 当前对话
    this.chatHistories = this.loadChatHistories();  // 所有历史
    
    this.initializeElements();      // 初始化DOM元素
    this.bindEvents();              // 绑定事件监听
    this.updateUI();                // 更新界面
    this.initMarkdown();            // Markdown渲染库初始化
    this.renderChatHistory();       // 渲染历史列表
}
```

### 3.3 对话模式

#### 模式1：快速对话 (quick)
```
用户输入
  ↓
POST /api/chat {Id, Question}
  ↓
等待完整响应
  ↓
一次性显示答案
  ↓
保存到会话历史
```

#### 模式2：流式对话 (stream)
```
用户输入
  ↓
POST /api/chat_stream {Id, Question}
  ↓
打开EventSource/SSE连接
  ↓
逐块接收数据
  ↓
实时渲染每个chunk
  ↓
流结束时更新完整历史
```

### 3.4 前端API集成

**关键方法**：

```javascript
// 发送聊天消息
async sendMessage(userQuery) {
    const requestBody = {
        Id: this.sessionId,
        Question: userQuery
    };
    
    if (this.currentMode === 'quick') {
        // 快速模式：一次性请求
        const response = await fetch(`${this.apiBaseUrl}/chat`, {
            method: 'POST',
            body: JSON.stringify(requestBody)
        });
        const data = await response.json();
        this.displayMessage(data.answer, 'assistant');
    } 
    else if (this.currentMode === 'stream') {
        // 流式模式：使用EventSource
        const eventSource = new EventSource(
            `${this.apiBaseUrl}/chat_stream?...`
        );
        eventSource.onmessage = (event) => {
            this.displayStreamChunk(event.data);
        };
    }
}

// 文件上传
async uploadFile(file) {
    const formData = new FormData();
    formData.append('file', file);
    
    const response = await fetch(`${this.apiBaseUrl}/upload`, {
        method: 'POST',
        body: formData
    });
    
    return response.json();  // {fileName, filePath, fileSize}
}

// AI运维分析
async triggerAIOps() {
    const response = await fetch(`${this.apiBaseUrl}/ai_ops`, {
        method: 'POST'
    });
    
    const data = await response.json();
    // 显示分析结果和详细步骤
}
```

### 3.5 UI交互逻辑

**消息显示**：
- 用户消息：右对齐，蓝色背景
- AI回复：左对齐，灰色背景
- Markdown渲染：代码块高亮、表格格式化

**对话历史**：
- 存储在LocalStorage中
- 每次对话生成UUID作为历史ID
- 侧边栏显示最近对话列表
- 点击可恢复历史对话

**加载状态**：
- 请求期间显示加载遮罩
- 提示："智能运维分析中，请稍候"
- 流式对话中逐步隐藏遮罩

---

## 四、完整数据流示例

### 4.1 快速对话流程

```
╔════════════════════════════════════════════════════════════════════╗
║                   用户快速对话完整流程                              ║
╚════════════════════════════════════════════════════════════════════╝

【前端】
User Input: "生产环境CPU告警怎么处理？"
  ↓
App.sendMessage() → POST /api/chat
  {
    "Id": "session_uuid_123",
    "Question": "生产环境CPU告警怎么处理？"
  }

【后端 - 控制层】
ControllerV1.Chat()
  ↓
【后端 - 业务逻辑】
构建UserMessage {
  ID: "session_uuid_123",
  Query: "生产环境CPU告警怎么处理？",
  History: [  // 从内存获取前面的对话
    UserMessage("前面的问题"),
    SystemMessage("前面的回答")
  ]
}

【后端 - AI管道】
BuildChatAgent() → 构建Eino Graph
  ↓
执行图操作：

┌─ InputToRag ─────────────────────────────────┐
│ input: UserMessage                           │
│ output: "生产环境CPU告警怎么处理？"            │
└─ → MilvusRetriever                          ┘
   • 向量化查询 (豆宝Embedding)
   • Milvus搜索 (TopK=1)
   • 返回: [相关文档块]
     "CPU告警处理步骤:
      1. 登录监控平台
      2. 查看进程占用
      3. 执行告警处理流程..."

┌─ InputToChat ──────────────────────────────────┐
│ input: UserMessage                            │
│ output: {                                      │
│   "content": "生产环境CPU告警怎么处理？",       │
│   "history": [前面的对话],                     │
│   "date": "2025-05-10 14:30:00"               │
│ }                                             │
└─ → ChatTemplate                              ┘
   • 组织System Prompt:
     "# 角色：对话小助手
      ... [系统提示词]
      ## 相关文档：
      CPU告警处理步骤:..."
   • 组织Chat History
   • 最终格式化消息

┌──────────────────────────────────────────────┐
│ ReactAgent (DeepSeek V3 Quick)               │
│ • 输入：格式化的完整上下文                     │
│ • 执行：                                      │
│   - 理解问题                                  │
│   - 参考检索文档                              │
│   - 可能调用搜索工具                          │
│   - 生成回复                                  │
│ • 输出：schema.Message                        │
│   Content: "根据内部文档，CPU告警处理流程是：
│            1. 首先检查当前CPU使用率...
│            2. 查看占用最高的进程...
│            3. 决定是否需要重启或优化..."
└──────────────────────────────────────────────┘

【后端 - 内存管理】
mem.GetSimpleMemory("session_uuid_123").SetMessages(
  UserMessage("生产环境CPU告警怎么处理？")
)
mem.GetSimpleMemory("session_uuid_123").SetMessages(
  SystemMessage("根据内部文档，CPU告警处理流程是...")
)

【返回前端】
ChatRes {
  "answer": "根据内部文档，CPU告警处理流程是：
            1. 首先检查当前CPU使用率...
            2. 查看占用最高的进程...
            3. 决定是否需要重启或优化..."
}

【前端展示】
显示AI回复在聊天区域
更新对话历史侧边栏
保存到LocalStorage
```

---

### 4.2 AI运维分析流程

```
╔════════════════════════════════════════════════════════════════════╗
║                   AI运维分析(Plan-Execute-Replan)流程              ║
╚════════════════════════════════════════════════════════════════════╝

【用户操作】
点击 "AI Ops" 按钮
  ↓
POST /api/ai_ops

【后端 - AIOps控制器】
ControllerV1.AIOps()
  ↓
构造任务Query:
"1. 获取所有活跃告警
 2. 查询每个告警的处理文档
 3. 查询相关日志
 4. 生成分析报告"

【执行 Plan-Execute-Replan】

┌─────────────────────────────────────────┐
│【第1阶段】Planner (DeepSeek V3思维)      │
└─────────────────────────────────────────┘
输入: Query字符串
  ↓
深度思考并规划:
"
计划步骤:
Step1: 调用query_prometheus_alerts获取告警列表
Step2: For each alert:
       - 调用query_internal_docs查询处理方案
       - 调用query_log查询相关日志
Step3: 汇总分析并生成报告
"
输出: Plan对象

┌─────────────────────────────────────────┐
│【第2阶段】Executor (DeepSeek V3 Quick)   │
└─────────────────────────────────────────┘
输入: Plan

执行步骤 (循环 ≤ 20次):

迭代1:
  工具调用: query_prometheus_alerts()
  返回: [
    {alertname: "HighCPU", state: "firing"},
    {alertname: "HighMemory", state: "firing"},
    {alertname: "DiskFull", state: "pending"}
  ]

迭代2-4: For each alert:
  工具调用: query_internal_docs("HighCPU告警处理")
  返回: "
    CPU告警处理方案:
    1. 登录监控平台
    2. 查看占用进程
    3. 执行以下处理...
  "

迭代5-7: 查询相关日志
  工具调用: query_log(query="HighCPU相关日志", region="ap-guangzhou")
  返回: [日志片段...]

【继续迭代...】

┌─────────────────────────────────────────┐
│【第3阶段】Replanner (重规划)              │
└─────────────────────────────────────────┘
• 检查上一步执行结果
• 评估是否完成所有计划
• 若未完成，修改计划继续迭代
• 若已完成或达到最大迭代数，结束

【生成最终输出】
最终Message Content:
"
告警分析报告
---
# 告警处理详情

## 活跃告警清单
1. HighCPU (firing 2h30m)
2. HighMemory (firing 1h15m)
3. DiskFull (pending 30m)

## 告警根因分析1 (HighCPU)
根据日志分析，该告警由以下原因触发：
- Java进程占用CPU 85%
- 原因：大量并发请求

## 处理方案执行1
按照内部文档流程:
1. 登录平台 ✓
2. 确认进程 ✓
3. 执行优化 ✓

[...继续其他告警...]

## 结论
已处理3个告警，建议继续监控。
"

【返回前端】
AIOpsRes {
  "result": "告警分析报告 ...",
  "detail": [
    "Step1: 查询告警 [成功]",
    "Step2: 查询CPU告警文档 [成功]",
    "...",
    "生成最终报告 [成功]"
  ]
}

【前端展示】
显示分析结果
展开详细步骤
```

---

## 五、关键技术点

### 5.1 RAG（检索增强生成）

**三个核心步骤**：

| 步骤 | 实现 | 作用 |
|------|------|------|
| 检索(Retrieve) | Milvus向量相似度搜索 | 从知识库找到相关文档 |
| 增强(Augment) | 将文档嵌入到Prompt | 提供LLM上下文 |
| 生成(Generate) | LLM根据上下文回答 | 基于知识库的准确回答 |

### 5.2 Plan-Execute-Replan模式

**核心思想**：将复杂任务分解为规划、执行、重规划的循环过程

**优势**：
- 处理复杂多步骤任务
- 动态调整执行策略
- 容错性和自适应性强
- 适用于运维自动化场景

### 5.3 工具调用集成

**架构特点**：
- 基于Eino框架的工具调用
- 支持函数调用（Function Calling）
- 工具结果自动注入到LLM上下文中
- 实现AI与外部系统的无缝集成

### 5.4 会话管理

**设计理念**：
- 基于会话ID的隔离存储
- 滑动窗口机制控制内存使用
- 保持对话连续性和上下文相关性
- 支持多并发对话场景

---

## 六、部署与配置

### 6.1 环境依赖

**后端**：
- Go 1.21+
- Milvus向量数据库
- Prometheus监控系统
- MySQL数据库
- 腾讯云CLS日志服务

**前端**：
- 现代浏览器（支持ES6+）
- 本地开发服务器（可选）

### 6.2 配置文件

**位置**：[hack/config.yaml](hack/config.yaml)

**关键配置项**：
```yaml
# AI模型配置
ds_think_chat_model:
  model: "deepseek-reasoner"
  api_key: "${DEEPSEEK_API_KEY}"
  base_url: "https://api.deepseek.com"

ds_quick_chat_model:
  model: "deepseek-chat"
  api_key: "${DEEPSEEK_API_KEY}"
  base_url: "https://api.deepseek.com"

# 向量数据库
milvus:
  address: "localhost:19530"
  collection: "knowledge_base"

# MCP服务器
mcp_server:
  url: "http://localhost:3000"
  region: "ap-guangzhou"
  topic_id: "869830db-a055-4479-963b-3c898d27e755"
```

### 6.3 启动命令

**后端服务**：
```bash
cd /path/to/project
go run main.go
# 服务启动在 http://localhost:6872
```

**前端开发**：
```bash
cd SuperBizAgentFrontend
python -m http.server 8000  # 或使用其他静态服务器
# 访问 http://localhost:8000
```

---

## 七、总结

**系统亮点**：

1. **AI能力集成**：结合RAG检索增强和Plan-Execute-Replan规划执行，实现了智能运维分析
2. **工具生态**：丰富的工具集支持，包括告警查询、日志分析、文档检索等
3. **架构设计**：清晰的分层架构，便于维护和扩展
4. **实时交互**：支持流式对话，提供良好的用户体验
5. **知识管理**：基于向量数据库的知识索引和检索系统

**核心价值**：
- 降低运维门槛，提高告警处理效率
- 提供24/7智能问答服务
- 支持复杂运维任务的自动化执行
- 持续学习和知识积累能力

这个系统代表了现代AI运维平台的典型架构，结合了最新的AI技术和传统的运维工具，为智能运维提供了完整的解决方案。
