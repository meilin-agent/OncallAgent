package plan_execute_replan

import (
	"context"
	"fmt"
	"strings"

	"SuperBizAgent/internal/ai/models"

	"github.com/cloudwego/eino/schema"
)

// GenerateFinalReport 兜底收尾：Plan-Execute-Replan 流程可能因模型未正确调用
// respond 工具而中断在中间状态（仅输出规划步骤 / 仅输出某个工具结果），
// 本函数基于全部执行记录强制生成最终分析报告，保证返回给用户的是稳定可读
// 的自然语言结果，而不是原始工具输出。
func GenerateFinalReport(ctx context.Context, query string, detail []string, fallback string) (string, error) {
	if len(detail) == 0 {
		return fallback, nil
	}

	cm, err := models.OpenAIForDeepSeekV3Quick(ctx)
	if err != nil {
		return "", fmt.Errorf("build report model failed: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("以下是智能运维分析流程中产生的全部执行记录，请基于这些记录生成最终的中文告警分析报告。\n")
	sb.WriteString("报告结构要求：\n")
	sb.WriteString("1. 活跃告警清单（含告警名称、触发接口/指标、持续时间等可获取的信息）\n")
	sb.WriteString("2. 告警根因分析\n")
	sb.WriteString("3. 处理方案（引用执行记录中检索到的内部文档步骤）\n")
	sb.WriteString("4. 结论\n")
	sb.WriteString("如果执行记录中没有查询到任何活跃告警，请如实说明当前没有活跃告警，不要编造。\n\n")
	for i, step := range detail {
		if len(step) > 4000 {
			step = step[:4000] + "...(内容过长已截断)"
		}
		fmt.Fprintf(&sb, "【执行记录 %d】\n%s\n", i+1, step)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		schema.SystemMessage("你是一个专业的运维告警分析助手，负责把 AI 自动排查过程的执行记录整理成结构清晰的中文分析报告。只基于给定的执行记录进行分析，不要编造不存在的告警或数据。"),
		schema.UserMessage(sb.String()),
	})
	if err != nil {
		return "", fmt.Errorf("generate final report failed: %w", err)
	}
	if strings.TrimSpace(resp.Content) == "" {
		return "", fmt.Errorf("generate final report returned empty content")
	}
	return resp.Content, nil
}
