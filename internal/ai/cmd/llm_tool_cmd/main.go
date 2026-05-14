package main

import (
	tools2 "SuperBizAgent/internal/ai/tools"
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

func main() {
	ctx := context.Background()
	// 创建 ChatModel

	model, err := g.Cfg().Get(ctx, "ds_think_chat_model.model")
	if err != nil {
		panic(err)
	}
	api_key, err := g.Cfg().Get(ctx, "ds_think_chat_model.api_key")
	if err != nil {
		panic(err)
	}
	base_url, err := g.Cfg().Get(ctx, "ds_think_chat_model.base_url")
	if err != nil {
		panic(err)
	}

	config := &openai.ChatModelConfig{
		APIKey:  api_key.String(),
		Model:   model.String(),
		BaseURL: base_url.String(),
	}
	chatModel, err := openai.NewChatModel(ctx, config)
	if err != nil {
		panic(err)
	}
	// 获取工具信息, 用于绑定到 ChatModel
	toolList, err := tools2.GetLogMcpTool()
	if err != nil {
		panic(err)
	}
	toolList = append(toolList, tools2.NewMysqlCrudTool())
	toolList = append(toolList, tools2.NewGetCurrentTimeTool())
	toolInfos := make([]*schema.ToolInfo, 0)
	var info *schema.ToolInfo
	for _, todoTool := range toolList {
		info, err = todoTool.Info(ctx)
		if err != nil {
			panic(err)
		}
		toolInfos = append(toolInfos, info)
	}

	// 将 tools 绑定到 ChatModel
	err = chatModel.BindTools(toolInfos)
	if err != nil {
		panic(err)
	}

	// 创建一个完整的处理链
	chain := compose.NewChain[[]*schema.Message, *schema.Message]()
	chain.AppendChatModel(chatModel, compose.WithNodeName("chat_model"))

	// 编译并运行 chain
	agent, err := chain.Compile(ctx)
	if err != nil {
		panic(err)
	}
	// 运行示例
	resp, err := agent.Invoke(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "告诉我你有哪些工具可以使用",
		},
	})
	if err != nil {
		panic(err)
	}
	// 输出结果
	fmt.Println(resp.Content)
}
