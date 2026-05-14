package main

import (
	"SuperBizAgent/internal/ai/agent/chat_pipeline"
	"SuperBizAgent/utility/mem"
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	id := "111"
	userMessage := &chat_pipeline.UserMessage{
		ID:      id,
		Query:   "你好，我叫nash",
		History: mem.GetSimpleMemory(id).GetMessages(),
	}
	runner, err := chat_pipeline.BuildChatAgent(ctx)
	if err != nil {
		panic(err)
	}
	// 第一次对话
	out, err := runner.Invoke(ctx, userMessage)
	if err != nil {
		panic(err)
	}
	answer := out.Content
	fmt.Println("Q: 你好，我叫nash")
	fmt.Println("A:", answer)
	mem.GetSimpleMemory(id).SetMessages(schema.UserMessage("你好，我叫nash"))
	mem.GetSimpleMemory(id).SetMessages(schema.AssistantMessage(out.Content, []schema.ToolCall{}))
	// 第二次对话
	userMessage = &chat_pipeline.UserMessage{
		ID:      id,
		Query:   "还记得我叫啥吗？我现在心情很不好，你能不能安慰我一下？请问当前时间是多少？",
		History: mem.GetSimpleMemory(id).GetMessages(),
	}
	out, err = runner.Invoke(ctx, userMessage)
	if err != nil {
		panic(err)
	}
	answer = out.Content
	fmt.Println("----------------")
	fmt.Println("Q: 还记得我叫啥吗？我现在心情很不好，你能不能安慰我一下？请问当前时间是多少？")
	fmt.Println("A:", answer)
}
