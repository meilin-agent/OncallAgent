package tools

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func NewEncourageTool() tool.InvokableTool {
	t, err := utils.InferOptionableTool(
		"encourage",
		"Encourage the user with a positive message. Use this tool when you want to provide motivation, support, or positive reinforcement to the user. The tool will return a randomly selected encouraging message to uplift the user's spirits.",
		func(ctx context.Context, input *struct{}, opts ...tool.Option) (output string, err error) {
			encouragements := []string{
				"Keep up the great work! You're doing amazing!",
				"Believe in yourself! You have the power to achieve anything!",
				"Every step you take is a step closer to your goals. Keep going!",
				"You are capable of incredible things. Don't give up!",
				"Your efforts are paying off! Stay positive and keep pushing forward!",
			}
			// 随机选择一个鼓励的话语
			output = encouragements[time.Now().UnixMilli()%int64(len(encouragements))]
			log.Printf("Encouragement tool called. Returning message: %s", output)
			return output, nil
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	return t
}
