package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

type ChatReq struct {
	g.Meta   `path:"/chat" method:"post" summary:"对话"`
	Id       string // 会话ID，用于关联聊天历史
	Question string // 用户提问内容
}

type ChatRes struct {
	Answer string `json:"answer"` // AI最终回答文本
}

type ChatStreamReq struct {
	g.Meta   `path:"/chat_stream" method:"post" summary:"流式对话"`
	Id       string // 会话ID
	Question string // 用户提问内容
}

type ChatStreamRes struct {
}

type FileUploadReq struct {
	g.Meta `path:"/upload" method:"post" mime:"multipart/form-data" summary:"文件上传"`
}

type FileUploadRes struct {
	FileName string `json:"fileName" dc:"保存的文件名"`   // 服务器保存后的文件名
	FilePath string `json:"filePath" dc:"文件保存路径"`   // 存储路径
	FileSize int64  `json:"fileSize" dc:"文件大小(字节)"` // 文件大小
}

type AIOpsReq struct {
	g.Meta `path:"/ai_ops" method:"post" summary:"AI运维"`
}

type AIOpsRes struct {
	Result string   `json:"result"` // AI运维分析结果文本
	Detail []string `json:"detail"` // 详细执行步骤或中间输出
}
