package chat_pipeline

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func BuildChatAgent(ctx context.Context) (r compose.Runnable[*UserMessage, *schema.Message], err error) {
	// 定义图中每个节点的名字
	const (
		InputToRag      = "InputToRag"
		ChatTemplate    = "ChatTemplate"
		ReactAgent      = "ReactAgent"
		MilvusRetriever = "MilvusRetriever"
		InputToChat     = "InputToChat"
	)

	// 创建可执行的图结构，输入为UserMessage，输出为schema.Message
	g := compose.NewGraph[*UserMessage, *schema.Message]()

	// 将用户消息转换为检索 查询字符串，供 MilvusRetriever 节点使用
	_ = g.AddLambdaNode(InputToRag, compose.InvokableLambdaWithOption(newInputToRagLambda), compose.WithNodeName("UserMessageToRag"))

	// 聊天提示词节点，负责组装System prompt、历史和检索结果，构建 []*schema.Message 数据格式，最后供 ReactAgent 节点使用
	chatTemplateKeyOfChatTemplate, err := newChatTemplate(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddChatTemplateNode(ChatTemplate, chatTemplateKeyOfChatTemplate)

	// 构建 ReAct Agent 节点，
	reactAgentKeyOfLambda, err := newReactAgentLambda(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddLambdaNode(ReactAgent, reactAgentKeyOfLambda, compose.WithNodeName("ReActAgent"))

	// 向量检索节点，查询Milvus返回相关文档
	milvusRetrieverKeyOfRetriever, err := newRetriever(ctx)
	if err != nil {
		return nil, err
	}
	// 注意下面的 output key 设置，把查询出来的设置为了documents，匹配 ChatTemplate 里面的 system prompt 中的占位符 {documents}
	_ = g.AddRetrieverNode(MilvusRetriever, milvusRetrieverKeyOfRetriever, compose.WithOutputKey("documents"))

	// 将用户消息转换为聊天上下文输入（历史、时间、content 等）
	_ = g.AddLambdaNode(InputToChat, compose.InvokableLambdaWithOption(newInputToChatLambda), compose.WithNodeName("UserMessageToChat"))

	// 定义节点间的执行顺序与依赖关系
	/*

		start -> InputToRag -> MilvusRetriever -> ChatTemplate  -> ReactAgent -> end
		start -> InputToChat                   -> ChatTemplate  -> ReactAgent -> end
	*/
	_ = g.AddEdge(compose.START, InputToRag)
	_ = g.AddEdge(compose.START, InputToChat)
	_ = g.AddEdge(ReactAgent, compose.END)
	_ = g.AddEdge(InputToRag, MilvusRetriever)
	_ = g.AddEdge(MilvusRetriever, ChatTemplate)
	_ = g.AddEdge(InputToChat, ChatTemplate)
	_ = g.AddEdge(ChatTemplate, ReactAgent)

	// 编译图，AllPredecessor 模式表示:等待所有前置节点执行完成。。例如：ChatTemplate 这个节点，会等待 InputToChat 和 InputToRag -> MilvusRetriever 执行完成，拿到它们的输出后才会执行。
	r, err = g.Compile(ctx, compose.WithGraphName("ChatAgent"), compose.WithNodeTriggerMode(compose.AllPredecessor))
	if err != nil {
		return nil, err
	}

	return r, err
}
