package main

import (
	"SuperBizAgent/internal/controller/chat"
	"SuperBizAgent/utility/common"
	"SuperBizAgent/utility/middleware"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"
)

func main() {
	// 创建GoFrame上下文，用于读取配置和初始化服务
	ctx := gctx.New()

	// 利用 manifest/config 夹下的 config.example.yaml 作为模板，创建一个新的 config.yaml 文件，并将里面需要的信息填写完整。程序会自动读取 config.yaml 中的配置信息。
	// 读取  manifest/config文件夹下的 config.yaml配置文件中，file_dir 的文件目录，用于后续文件上传等操作
	fileDir, err := g.Cfg().Get(ctx, "file_dir")
	if err != nil {
		panic(err)
	}

	common.FileDir = fileDir.String()

	// 创建HTTP服务器实例并挂载路由
	s := g.Server()
	s.Group("/api", func(group *ghttp.RouterGroup) {
		// 全局中间件：CORS和统一响应格式
		group.Middleware(middleware.CORSMiddleware)
		group.Middleware(middleware.ResponseMiddleware)
		// 将 chat 控制器绑定到 /api 路径下
		group.Bind(chat.NewV1())
	})

	// 设置服务端口并启动
	s.SetPort(6872)
	s.Run()
}
