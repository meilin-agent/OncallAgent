package tools

import (
	"context"

	e_mcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

/*
	GetLogMcpTool

如果 cls 这个产品还没有开通，记得先开通下 https://cloud.tencent.com/product/cls?from_cn_redirect=1

https://console.cloud.tencent.com/cam/capi 登录自己的腾讯云，在这里获取 API秘钥
https://cloud.tencent.com/developer/mcp/server/11710 进入这个页面，在右边，填写API密钥信息，传输方式用那个默认的 stdio即可。 然后点击链接。即可申请一个新的MCP URL，拿到这个URL后替换掉代码中的mcp_url变量值，就可以使用这个工具了。
*/
func GetLogMcpTool() ([]tool.BaseTool, error) {
	// https://mcp-api.tencent-cloud.com/sse/XXXX
	mcp_url := "https://mcp-api.tencent-cloud.com/sse/02807673a2788326"
	if mcp_url == "" {
		panic("要使用GetLogMcpTool，请先按照教程申请mcp url")
	}

	ctx := context.Background()
	cli, err := client.NewSSEMCPClient(mcp_url)
	if err != nil {
		return []tool.BaseTool{}, err
	}

	err = cli.Start(ctx)
	if err != nil {
		return []tool.BaseTool{}, err
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "example-client",
		Version: "1.0.0",
	}
	if _, err = cli.Initialize(ctx, initRequest); err != nil {
		return []tool.BaseTool{}, err
	}

	mcpTools, err := e_mcp.GetTools(ctx, &e_mcp.Config{Cli: cli})
	if err != nil {
		return []tool.BaseTool{}, err
	}
	return mcpTools, nil
}
