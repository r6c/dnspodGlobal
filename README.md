# DNSPod libdns Provider

A [libdns](https://github.com/libdns/libdns) provider for DNSPod DNS service.

## ⚠️ 重要的API要求 / Important API Requirements

### UserAgent 设置
DNSPod API 要求必须设置正确格式的 UserAgent，否则账户将被封禁：
- **格式要求**: `程序英文名称/版本(联系邮箱)`
- **示例**: `libdns-dnspod/1.0.0 (github.com/r6c/dnspodGlobal)`
- **已修复**: ✅ 当前代码使用正确的 UserAgent 格式

### 其他API规范
- ✅ 使用 HTTPS (`https://dnsapi.cn/`)
- ✅ 仅支持 POST 方法
- ✅ UTF-8 编码
- ✅ login_token 认证格式：`ID,Token`
- ✅ 中文线路名称："默认"

## 安装

```bash
go get github.com/r6c/dnspodGlobal
```

## 使用方法

### 环境变量设置
```bash
export DNSPOD_TOKEN="your_id,your_token"
export ZONE="your-domain.com"
```

### 代码示例
```go
package main

import (
	"context"
	"time"
	
	dnspod "github.com/r6c/dnspodGlobal"
	"github.com/libdns/libdns"
)

func main() {
	provider := dnspod.Provider{
		LoginToken: "your_id,your_token",
	}
	
	// 获取记录
	records, err := provider.GetRecords(context.TODO(), "example.com")
	
	// 添加记录
	newRecords, err := provider.AppendRecords(context.TODO(), "example.com", []libdns.Record{
		libdns.TXT{
			Name: "test.example.com.",
			Text: "Hello World",
			TTL:  600 * time.Second,
		},
	})
	
	// 更新记录
	updatedRecords, err := provider.SetRecords(context.TODO(), "example.com", newRecords)
	
	// 删除记录
	deletedRecords, err := provider.DeleteRecords(context.TODO(), "example.com", updatedRecords)
}
```

## 支持的记录类型

- A/AAAA (使用 `libdns.Address`)
- TXT (使用 `libdns.TXT`) 
- CNAME (使用 `libdns.CNAME`)
- MX (使用 `libdns.MX`)
- 其他类型 (使用 `libdns.RR`)

## API Token 获取

1. 登录 [DNSPod 控制台](https://console.dnspod.cn/)
2. 进入 [密钥管理](https://console.dnspod.cn/account/token) 页面
3. 创建新的 API Token
4. 格式为：`ID,Token` (用逗号分隔)

## 运行示例

```bash
DNSPOD_TOKEN="your_id,your_token" ZONE="your-domain.com" go run _example/main.go
```

## 注意事项

⚠️ **避免API滥用**: DNSPod对API使用有严格限制，请避免：
- 短时间内大量操作
- 无变化的重复请求
- 程序死循环
- 用于大量测试

违反将导致账户被临时封禁（通常1小时）。

## 许可证

MIT License
