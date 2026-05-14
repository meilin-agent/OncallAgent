package chat

import (
	v1 "SuperBizAgent/api/chat/v1"
	"SuperBizAgent/internal/ai/agent/knowledge_index_pipeline"
	loader2 "SuperBizAgent/internal/ai/loader"
	"SuperBizAgent/utility/client"
	"SuperBizAgent/utility/common"
	"SuperBizAgent/utility/log_call_back"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

func (c *ControllerV1) FileUpload(ctx context.Context, req *v1.FileUploadReq) (res *v1.FileUploadRes, err error) {
	// 从请求中获取上传的文件
	r := g.RequestFromCtx(ctx)
	uploadFile := r.GetUploadFile("file")
	if uploadFile == nil {
		return nil, gerror.New("请上传文件")
	}

	// 确保保存目录存在
	if !gfile.Exists(common.FileDir) {
		if err := gfile.Mkdir(common.FileDir); err != nil {
			return nil, gerror.Wrapf(err, "创建目录失败: %s", common.FileDir)
		}
	}

	// 获取原始文件名
	newFileName := uploadFile.Filename
	// 完整的保存路径
	savePath := filepath.Join(common.FileDir)

	// 保存文件
	_, err = uploadFile.Save(savePath, false)
	if err != nil {
		return nil, gerror.Wrapf(err, "保存文件失败")
	}

	// 获取文件信息
	fileInfo, err := os.Stat(savePath)
	if err != nil {
		return nil, gerror.Wrapf(err, "获取文件信息失败")
	}

	res = &v1.FileUploadRes{
		FileName: newFileName,
		FilePath: savePath,
		FileSize: fileInfo.Size(),
	}
	err = buildIntoIndex(ctx, common.FileDir+"/"+newFileName)
	if err != nil {
		return nil, gerror.Wrapf(err, "构建知识库失败")
	}
	return res, nil
}

func buildIntoIndex(ctx context.Context, path string) error {

	r, err := knowledge_index_pipeline.BuildKnowledgeIndexing(ctx)
	// 删除biz数据metadata中_source一样的数据
	loader, err := loader2.NewFileLoader(ctx)
	if err != nil {
		return err
	}
	docs, err := loader.Load(ctx, document.Source{URI: path})
	if err != nil {
		return err
	}
	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		return err
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

}
