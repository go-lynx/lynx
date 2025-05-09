package base

import (
	"os"

	"golang.org/x/mod/modfile"
)

// ModulePath 从指定的 go.mod 文件中提取 Go 模块路径。
// 参数 filename 是 go.mod 文件的路径。
// 返回值:
//   - string: 提取到的 Go 模块路径。
//   - error: 若读取文件或解析模块路径时出错，则返回相应的错误信息；否则返回 nil。
func ModulePath(filename string) (string, error) {
	// 读取指定路径的 go.mod 文件内容
	modBytes, err := os.ReadFile(filename)
	if err != nil {
		// 若读取文件失败，返回空字符串和错误信息
		return "", err
	}
	// 调用 modfile.ModulePath 函数从文件内容中提取 Go 模块路径
	return modfile.ModulePath(modBytes), nil
}
