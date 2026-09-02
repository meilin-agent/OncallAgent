package plan_execute_replan

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-examples/adk/common/prints"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
)

func BuildPlanAgent(ctx context.Context, query string) (string, []string, error) {
	// 构建三阶段Agent：Planner、Executor、Replanner
	planAgent, err := NewPlanner(ctx)
	if err != nil {
		return "", []string{}, err
	}
	executeAgent, err := NewExecutor(ctx)
	if err != nil {
		return "", []string{}, err
	}
	replanAgent, err := NewRePlanAgent(ctx)
	if err != nil {
		return "", []string{}, err
	}

	planExecuteAgent, err := planexecute.New(ctx, &planexecute.Config{
		Planner:       planAgent,
		Executor:      executeAgent,
		Replanner:     replanAgent,
		MaxIterations: 20,
	})
	if err != nil {
		return "", []string{}, fmt.Errorf("build PlanExecuteAgent Error: %v", err)
	}

	// 创建运行器，依次执行复杂任务迭代过程
	r := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: planExecuteAgent,
	})
	iter := r.Query(ctx, query)
	var lastMessage adk.Message
	var detail []string

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		// 这里打印事件，便于调试和查看每次迭代结果
		fmt.Println("------------- Event -------------")
		prints.Event(event)

		if event.Output != nil {
			lastMessage, _, err = adk.GetMessage(event)
			detail = append(detail, lastMessage.String())
		}
	}

	if lastMessage == nil {
		return "", []string{}, fmt.Errorf("get lastMessage Error")
	}

	// 兜底收尾：流程可能因模型未正确调用 respond 工具而中断在中间状态，
	// 基于全部执行记录强制生成最终报告，失败时回退原始结果
	content := lastMessage.Content
	if report, err := GenerateFinalReport(ctx, query, detail, content); err == nil {
		content = report
	} else {
		fmt.Printf("[warn] generate final report failed, fallback to raw result: %v\n", err)
	}

	// 返回最终生成的内容和所有步骤详情
	return content, detail, nil
}
