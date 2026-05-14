package indexer

import (
	embedder2 "SuperBizAgent/internal/ai/embedder"
	"SuperBizAgent/utility/client"
	"SuperBizAgent/utility/common"
	"context"
	"encoding/json"
	"fmt"

	milvus2 "github.com/cloudwego/eino-ext/components/indexer/milvus2"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
)

func NewMilvusIndexer(ctx context.Context) (*milvus2.Indexer, error) {
	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		return nil, err
	}
	eb, err := embedder2.OllamaEmbedding(ctx)
	if err != nil {
		return nil, err
	}
	config := &milvus2.IndexerConfig{
		Client:     cli,
		Collection: common.MilvusCollectionName,
		Vector: &milvus2.VectorConfig{
			Dimension:    768, // 与 embedding 模型维度匹配
			MetricType:   milvus2.COSINE,
			IndexBuilder: milvus2.NewHNSWIndexBuilder().WithM(16).WithEfConstruction(200),
		},
		Embedding:         eb,
		DocumentConverter: floatVectorDocumentConverter,
	}
	indexer, err := milvus2.NewIndexer(ctx, config)
	if err != nil {
		return nil, err
	}
	return indexer, nil
}

func floatVectorDocumentConverter(
	ctx context.Context,
	docs []*schema.Document,
	vectors [][]float64,
) ([]column.Column, error) {

	if len(docs) != len(vectors) {
		return nil, fmt.Errorf(
			"docs count %d != vectors count %d",
			len(docs),
			len(vectors),
		)
	}

	ids := make([]string, 0, len(docs))
	contents := make([]string, 0, len(docs))
	metadataList := make([][]byte, 0, len(docs))
	floatVectors := make([][]float32, 0, len(docs))

	for i, doc := range docs {

		// id
		id := doc.ID
		if id == "" {
			id = fmt.Sprintf("doc_%d", i)
		}

		ids = append(ids, id)

		// content
		contents = append(contents, doc.Content)

		// metadata
		metaBytes, err := json.Marshal(doc.MetaData)
		if err != nil {
			return nil, err
		}

		metadataList = append(metadataList, metaBytes)

		// vector
		vec64 := vectors[i]

		vec32 := make([]float32, len(vec64))
		for j, v := range vec64 {
			vec32[j] = float32(v)
		}

		floatVectors = append(floatVectors, vec32)
	}

	columns := []column.Column{
		column.NewColumnVarChar(
			"id",
			ids,
		),

		column.NewColumnVarChar(
			"content",
			contents,
		),

		column.NewColumnJSONBytes(
			"metadata",
			metadataList,
		),

		column.NewColumnFloatVector(
			"vector",
			768,
			floatVectors,
		),
	}

	return columns, nil
}
