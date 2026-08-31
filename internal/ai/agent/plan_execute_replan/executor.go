package plan_execute_replan

import (
	"SuperBizAgent/internal/ai/models"
	"SuperBizAgent/internal/ai/tools"
	"context"
	"log"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

func NewExecutor(ctx context.Context) (adk.Agent, error) {
	// log（MCP 日志工具为可选能力：连接失败时降级跳过，不影响其余工具）
	mcpTool, err := tools.GetLogMcpTool()
	if err != nil {
		log.Printf("[warn] log MCP tool unavailable, degrade to continue without it: %v", err)
		mcpTool = []tool.BaseTool{}
	}
	toolList := mcpTool
	// alerts
	toolList = append(toolList, tools.NewPrometheusAlertsQueryTool())
	// file
	toolList = append(toolList, tools.NewQueryInternalDocsTool())
	// time
	toolList = append(toolList, tools.NewGetCurrentTimeTool())
	execModel, err := models.OpenAIForDeepSeekV3Quick(ctx)
	if err != nil {
		return nil, err
	}
	return planexecute.NewExecutor(ctx, &planexecute.ExecutorConfig{
		Model: execModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: toolList,
			},
		},
		// 内层 ReAct 循环必须有界：无上限时模型在工具间反复打转，
		// 上下文无限增长导致输出退化、请求长时间不返回
		MaxIterations: 10,
	})
}
