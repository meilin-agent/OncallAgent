package models

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/gogf/gf/v2/frame/g"
)

func OpenAIForDeepSeekV31Think(ctx context.Context) (cm model.ToolCallingChatModel, err error) {

	model, err := g.Cfg().Get(ctx, "ds_think_chat_model.model")
	if err != nil {
		return nil, err
	}
	api_key, err := g.Cfg().Get(ctx, "ds_think_chat_model.api_key")
	if err != nil {
		return nil, err
	}
	base_url, err := g.Cfg().Get(ctx, "ds_think_chat_model.base_url")
	if err != nil {
		return nil, err
	}

	config := &openai.ChatModelConfig{
		Model:   model.String(),
		APIKey:  api_key.String(),
		BaseURL: base_url.String(),
	}

	cm, err = openai.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}
	return cm, nil
}

func OpenAIForDeepSeekV3Quick(ctx context.Context) (cm model.ToolCallingChatModel, err error) {

	model, err := g.Cfg().Get(ctx, "ds_quick_chat_model.model")
	if err != nil {
		return nil, err
	}
	api_key, err := g.Cfg().Get(ctx, "ds_quick_chat_model.api_key")
	if err != nil {
		return nil, err
	}
	// 这里本质是访问火山引擎提供的 中的模型接口，所以base_url是火山引擎提供的地址，而不是微软和OpenAI公司云平台提供服务提供的 Azure OpenAI Service 的地址
	base_url, err := g.Cfg().Get(ctx, "ds_quick_chat_model.base_url")
	if err != nil {
		return nil, err
	}

	temperature := float32(0.3) // 可以根据需要调整温度参数
	top_p := float32(0.9)       // 可以根据需要调整top_p参数
	// openai库 应该是早期通用的一种连接各个平台LLM的方式
	config := &openai.ChatModelConfig{
		Model:       model.String(),
		APIKey:      api_key.String(),
		BaseURL:     base_url.String(),
		Temperature: &temperature,
		TopP:        &top_p,
	}

	// 创建 大语言模型实例
	cm, err = openai.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}
	return cm, nil
}

// OpenAIForDeepSeekV3Quick 的写法其实和这种写法是一样的。这里是直接使用的 Ark库，直接访问火山引擎
// 代码只是作为演示写法，功能应该也是ok的，具体可以参考连接各个平台的demo  https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/chat_model/
func ArkForDeepSeekV3Quick(ctx context.Context) (cm model.ToolCallingChatModel, err error) {
	// 从配置中读取快速响应模型参数
	model, err := g.Cfg().Get(ctx, "ds_quick_chat_model.model")
	if err != nil {
		return nil, err
	}
	api_key, err := g.Cfg().Get(ctx, "ds_quick_chat_model.api_key")
	if err != nil {
		return nil, err
	}
	// 不需要这个配置，ark库中有默认的地址
	// base_url, err := g.Cfg().Get(ctx, "ds_quick_chat_model.base_url")
	// if err != nil {
	// 	return nil, err
	// }
	config := &ark.ChatModelConfig{ // 区别
		Model:  model.String(),
		APIKey: api_key.String(),
	}

	// 创建快速聊天模型
	cm, err = ark.NewChatModel(ctx, config) // 区别
	if err != nil {
		return nil, err
	}
	return cm, nil
}
