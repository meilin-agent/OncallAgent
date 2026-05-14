package embedder

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
	"github.com/cloudwego/eino-ext/components/embedding/ollama"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/gogf/gf/v2/frame/g"
)

func DoubaoEmbedding(ctx context.Context) (eb embedding.Embedder, err error) {
	model, err := g.Cfg().Get(ctx, "doubao_embedding_model.model") // 读取配置模型
	if err != nil {
		return nil, err
	}
	api_key, err := g.Cfg().Get(ctx, "doubao_embedding_model.api_key")
	if err != nil {
		return nil, err
	}
	dim := 2048
	embedder, err := dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{
		Model:      model.String(),
		APIKey:     api_key.String(),
		Dimensions: &dim,
	})
	if err != nil {
		log.Printf("new embedder error: %v\n", err)
		return nil, err
	}
	return embedder, nil
}

func OllamaEmbedding(ctx context.Context) (eb embedding.Embedder, err error) {
	model, err := g.Cfg().Get(ctx, "ollama_embedding_model.model") // 读取配置模型
	if err != nil {
		return nil, err
	}

	base_url, err := g.Cfg().Get(ctx, "ollama_embedding_model.base_url") // 读取配置模型
	if err != nil {
		return nil, err
	}

	embedder, err := ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{
		BaseURL: base_url.String(),
		Model:   model.String(),
		Timeout: 10 * time.Second,
	})
	if err != nil {
		log.Fatalf("NewEmbedder of ollama error: %v", err)
		return nil, err
	}
	return embedder, nil
}
