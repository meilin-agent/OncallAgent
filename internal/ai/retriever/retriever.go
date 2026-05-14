package retriever

import (
	"SuperBizAgent/internal/ai/embedder"
	"SuperBizAgent/utility/client"
	"SuperBizAgent/utility/common"
	"context"
	"log"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
	"github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
	"github.com/cloudwego/eino/components/retriever"
)

func NewMilvusRetriever(ctx context.Context) (rtr retriever.Retriever, err error) {

	//	创建 Milvue 客户端
	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create Milvus client: %v", err)
		return nil, err
	}
	log.Printf("Milvus client created successfully")

	// 创建文本嵌入器
	eb, err := embedder.OllamaEmbedding(ctx)
	if err != nil {
		return nil, err
	}

	// 创建 retriever
	r, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		Client:      cli,
		VectorField: "vector",
		Collection:  common.MilvusCollectionName,
		TopK:        3,
		SearchMode:  search_mode.NewApproximate(milvus2.L2),
		Embedding:   eb,
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
		return nil, err
	}
	log.Printf("Retriever created successfully")

	return r, nil

}
