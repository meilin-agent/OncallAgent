package client

import (
	"SuperBizAgent/utility/common"
	"context"
	"fmt"

	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

func NewMilvusClient(ctx context.Context) (*milvusclient.Client, error) {

	// 1. 先连接default数据库
	defaultClient, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: "localhost:19530",
		DBName:  "default",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to default database: %w", err)
	}
	// 2. 检查agent数据库是否存在，不存在则创建
	databases, err := defaultClient.ListDatabase(ctx, milvusclient.NewListDatabaseOption())
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	agentDBExists := false
	for _, dbName := range databases {
		if dbName == common.MilvusDBName {
			agentDBExists = true
			break
		}
	}
	if !agentDBExists {
		err = defaultClient.CreateDatabase(ctx, milvusclient.NewCreateDatabaseOption(common.MilvusDBName)) // 创建agent数据库
		if err != nil {
			return nil, fmt.Errorf("failed to create agent database: %w", err)
		}
	}

	// 3. 创建连接到agent数据库的客户端
	agentClient, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: "localhost:19530",
		DBName:  common.MilvusDBName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent database: %w", err)
	}

	// 4. 检查 名字为biz 的collection是否存在，不存在则创建
	collections, err := agentClient.ListCollections(ctx, milvusclient.NewListCollectionOption())
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}
	bizCollectionExists := false
	for _, collectionName := range collections {
		if collectionName == common.MilvusCollectionName {
			bizCollectionExists = true
			break
		}
	}

	if !bizCollectionExists {
		// 创建biz collection的schema
		schema := entity.NewSchema()
		schema.WithName(common.MilvusCollectionName)
		schema.WithDescription("Business knowledge collection")
		// 启用动态字段支持，这样可以输出额外的字段
		schema.WithDynamicFieldEnabled(true)

		schema.WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeVarChar).WithMaxLength(256).WithIsPrimaryKey(true).WithIsAutoID(true))
		schema.WithField(entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(768))
		schema.WithField(entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(8192))
		schema.WithField(entity.NewField().WithName("metadata").WithDataType(entity.FieldTypeJSON))

		err = agentClient.CreateCollection(ctx, milvusclient.NewCreateCollectionOption(common.MilvusCollectionName, schema)) // 创建biz collection
		if err != nil {
			return nil, fmt.Errorf("failed to create biz collection: %w", err)
		}

		fmt.Printf("Created collection with schema fields:\n")
		for _, field := range schema.Fields {
			fmt.Printf("  Field: %s, Type: %v, TypeParams: %v\n", field.Name, field.DataType, field.TypeParams)
		}

		// 为id字段创建autoindex索引
		idIndex := index.NewAutoIndex(entity.L2)
		if err != nil {
			return nil, fmt.Errorf("failed to create id index: %w", err)
		}
		task, err := agentClient.CreateIndex(ctx, milvusclient.NewCreateIndexOption(common.MilvusCollectionName, "id", idIndex))
		if err != nil {
			return nil, fmt.Errorf("failed to create id index: %w", err)
		}
		task.Await(ctx)

		// 为 vector字段创建  HNSWIndex 索引
		vectorIndex := index.NewHNSWIndex(entity.L2, 16, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to create vector index: %w", err)
		}
		task, err = agentClient.CreateIndex(ctx, milvusclient.NewCreateIndexOption(common.MilvusCollectionName, "vector", vectorIndex))
		if err != nil {
			return nil, fmt.Errorf("failed to create vector index: %w", err)
		}
		task.Await(ctx)
	}

	// 主动 将 collection 加载到内存
	loadTask, err := agentClient.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(common.MilvusCollectionName))
	if err != nil {
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}
	err = loadTask.Await(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}

	// 关闭default数据库连接
	defaultClient.Close(ctx)

	return agentClient, nil
}
