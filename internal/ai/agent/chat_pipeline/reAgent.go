package chat_pipeline

import (
	"SuperBizAgent/internal/ai/tools"
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
)

func newReactAgentLambda(ctx context.Context) (lba *compose.Lambda, err error) {
	// 配置 ReAct Agent 的参数，包括最大步骤数、工具调用模型和可用工具等
	config := &react.AgentConfig{
		MaxStep:            25,
		ToolReturnDirectly: map[string]struct{}{}, // 这里可以配置哪些工具的输出需要直接返回给用户，而不是继续输入到模型中
	}

	// 初始化具有工具调用能力的 LLM 实例，供 ReAct Agent 使用
	chatModel, err := newChatModel(ctx)
	if err != nil {
		return nil, err
	}
	// ToolCallingModel 需要具有工具调用能力的 Model
	config.ToolCallingModel = chatModel
	//searchTool, err := newSearchTool(ctx)
	//if err != nil {
	//	return nil, err
	//}

	// 在添加工具
	// mcpTool, err := tools.GetLogMcpTool()
	// if err != nil {
	// 	return nil, err
	// }
	// // 在添加工具
	// config.ToolsConfig.Tools = mcpTool
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewPrometheusAlertsQueryTool())
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewMysqlCrudTool())
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewGetCurrentTimeTool())
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewQueryInternalDocsTool())
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewEncourageTool())
	// 创建 ReAct Agent 实例
	ins, err := react.NewAgent(ctx, config)
	if err != nil {
		return nil, err
	}

	// 将 ReAct Agent 包装成一个可执行的 Lambda，供图中的节点调用
	lba, err = compose.AnyLambda(ins.Generate, ins.Stream, nil, nil)
	if err != nil {
		return nil, err
	}

	return lba, nil
}
