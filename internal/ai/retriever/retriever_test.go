package retriever

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"
	"time"
)

const milvusAddress = "localhost:19530"

// TestMilvusRetriever 集成测试：依赖本地运行的 Milvus（docker-compose 启动）。
// Milvus 不可达时自动跳过，保证 CI/无基础设施环境下 go test ./... 可以通过。
func TestMilvusRetriever(t *testing.T) {

	ctx := context.Background()

	// 快速探测 Milvus 是否可用，避免无 Milvus 环境挂起至连接超时
	conn, err := net.DialTimeout("tcp", milvusAddress, time.Second)
	if err != nil {
		t.Skipf("Milvus not reachable at %s, skipping integration test: %v", milvusAddress, err)
	}
	conn.Close()

	rt, err := NewMilvusRetriever(ctx)
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
		return
	}

	// Retrieve documents
	documents, err := rt.Retrieve(ctx, "milvus")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
		return
	}

	// Print the documents
	for i, doc := range documents {
		fmt.Printf("Document %d:\n", i)
		fmt.Printf("title: %s\n", doc.ID)
		fmt.Printf("content: %s\n", doc.Content)
		fmt.Printf("metadata: %v\n", doc.MetaData)
	}
}
