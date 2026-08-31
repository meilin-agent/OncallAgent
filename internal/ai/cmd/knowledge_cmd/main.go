package main

import (
	"SuperBizAgent/internal/ai/agent/knowledge_index_pipeline"
	"SuperBizAgent/utility/client"
	"SuperBizAgent/utility/common"
	"SuperBizAgent/utility/log_call_back"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	loader2 "SuperBizAgent/internal/ai/loader"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

func main() {
	ctx := context.Background()
	r, err := knowledge_index_pipeline.BuildKnowledgeIndexing(ctx)
	if err != nil {
		panic(err)
	}

	loader, err := loader2.NewFileLoader(ctx)
	if err != nil {
		panic(err)
	}

	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		panic(err)
	}
	defer cli.Close(ctx)

	err = filepath.WalkDir("./docs", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk dir failed: %w", walkErr)
		}
		if d.IsDir() {
			return nil
		}
		// 统一使用斜杠分隔符：Windows 下 WalkDir 返回反斜杠路径，
		// 会导致 Milvus 过滤表达式 metadata["_source"] == "docs\bak\x.md" 解析失败
		path = filepath.ToSlash(path)

		if !strings.HasSuffix(path, ".md") {
			fmt.Printf("[skip] not a markdown file: %s\n", path)
			return nil
		}

		fmt.Printf("[start] indexing file: %s\n", path)

		docs, err := loader.Load(ctx, document.Source{URI: path})
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			fmt.Printf("[skip] no docs loaded from: %s\n", path)
			return nil
		}

		// 查询所有 metadata 中 _source 一样的数据并删除
		expr := fmt.Sprintf(`metadata["_source"] == "%s"`, docs[0].MetaData["_source"])
		queryOpt := milvusclient.NewQueryOption(common.MilvusCollectionName).
			WithFilter(expr).
			WithOutputFields("id")
		queryResult, err := cli.Query(ctx, queryOpt)
		if err != nil {
			return err
		}

		if queryResult.Len() > 0 {
			idCol := queryResult.GetColumn("id")
			if idCol == nil {
				return fmt.Errorf("milvus query returned %d rows but missing output field 'id'", queryResult.Len())
			}

			idsToDelete := make([]string, 0, idCol.Len())
			for i := 0; i < idCol.Len(); i++ {
				id, err := idCol.GetAsString(i)
				if err != nil {
					continue
				}
				idsToDelete = append(idsToDelete, id)
			}

			if len(idsToDelete) > 0 {
				deleteOpt := milvusclient.NewDeleteOption(common.MilvusCollectionName).WithStringIDs("id", idsToDelete)
				_, err = cli.Delete(ctx, deleteOpt)
				if err != nil {
					fmt.Printf("[warn] delete existing data failed: %v\n", err)
				} else {
					fmt.Printf("[info] deleted %d existing records with _source: %s\n", len(idsToDelete), docs[0].MetaData["_source"])
				}
			}
		}

		// 重新构建
		ids, err := r.Invoke(ctx, document.Source{URI: path}, compose.WithCallbacks(log_call_back.LogCallback(nil)))
		if err != nil {
			fmt.Printf("[error] invoke index graph failed: %v\n", err)
			return err
		}
		fmt.Printf("[done] indexing file: %s, len of parts: %d，%s\n", path, len(ids), ids)
		return nil
	})
	if err != nil {
		panic(err)
	}
}
