package knowledge_index_pipeline

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// TestNewDocumentTransformer_SplitsByH1Headers 按一级标题切分，并写入 title 元数据与 UUID 文档 ID
func TestNewDocumentTransformer_SplitsByH1Headers(t *testing.T) {
	ctx := context.Background()
	tfr, err := newDocumentTransformer(ctx)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	src := `# 服务下线
告警解释：服务可能因为 panic 导致 pod 重启。
解决方案：
1. 根据关键字 panic 搜索最近 1 小时日志。
2. 根据 panic 日志分析原因。

# 接口失败率过高
告警解释：接口失败率过高可能由下游服务不可用导致。
解决方案：
1. 根据接口名和 response 关键字搜索日志。
2. 根据日志 error 分析原因。`

	docs, err := tfr.Transform(ctx, []*schema.Document{{Content: src}})
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs split by '#' headers, got %d", len(docs))
	}

	for i, doc := range docs {
		title, ok := doc.MetaData["title"]
		if !ok || title == "" {
			t.Fatalf("doc %d missing title metadata: %v", i, doc.MetaData)
		}
		if !uuidRe.MatchString(doc.ID) {
			t.Fatalf("doc %d id %q is not a UUID", i, doc.ID)
		}
	}
	if docs[0].MetaData["title"] != "服务下线" {
		t.Fatalf("unexpected title for doc 0: %v", docs[0].MetaData["title"])
	}
	if docs[1].MetaData["title"] != "接口失败率过高" {
		t.Fatalf("unexpected title for doc 1: %v", docs[1].MetaData["title"])
	}
	if !strings.Contains(docs[0].Content, "告警解释") {
		t.Fatalf("doc 0 content should contain its section body, got: %q", docs[0].Content)
	}
}

// TestNewDocumentTransformer_NoHeaders_KeepsSingleDoc 无标题的文档整体保留为单个分块
func TestNewDocumentTransformer_NoHeaders_KeepsSingleDoc(t *testing.T) {
	ctx := context.Background()
	tfr, err := newDocumentTransformer(ctx)
	if err != nil {
		t.Fatalf("failed to create transformer: %v", err)
	}

	src := "这是一段没有标题的纯文本。\n它不应该被拆分。"
	docs, err := tfr.Transform(ctx, []*schema.Document{{Content: src}})
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc for headerless text, got %d", len(docs))
	}
	if _, ok := docs[0].MetaData["title"]; ok {
		t.Fatalf("headerless text should carry no title metadata, got: %v", docs[0].MetaData)
	}
	if !strings.Contains(docs[0].Content, "纯文本") {
		t.Fatalf("content should be preserved, got: %q", docs[0].Content)
	}
}
