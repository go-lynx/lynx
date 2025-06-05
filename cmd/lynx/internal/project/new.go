package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"

	"github.com/go-lynx/lynx/cmd/lynx/internal/base"
)

// Project 表示一个项目模板，包含项目的名称和路径信息。
type Project struct {
	Name string // 项目名称
	Path string // 项目路径
}

// New 从远程仓库创建一个新项目。
// ctx: 上下文，用于控制操作的生命周期。
// dir: 项目创建的目标目录。
// layout: 项目布局的远程仓库地址。
// branch: 要使用的远程仓库分支。
// 返回值: 若操作过程中出现错误，则返回相应的错误信息；否则返回 nil。
func (p *Project) New(ctx context.Context, dir string, layout string, branch string) error {
	// 计算项目最终创建的完整路径
	to := filepath.Join(dir, p.Name)

	// 检查目标路径是否已存在
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		// 若存在，提示用户路径已存在
		fmt.Printf("🚫 %s already exists\n", p.Name)
		// 创建一个确认提示，询问用户是否要覆盖该文件夹
		prompt := &survey.Confirm{
			Message: "📂 Do you want to override the folder ?",
			Help:    "Delete the existing folder and create the project.",
		}
		var override bool
		// 询问用户并将结果存储在 override 变量中
		e := survey.AskOne(prompt, &override)
		if e != nil {
			return e
		}
		// 若用户不同意覆盖，则返回错误
		if !override {
			return err
		}
		// 删除已存在的文件夹
		err := os.RemoveAll(to)
		if err != nil {
			return err
		}
	}

	// 提示用户开始创建项目，并显示项目名称和布局仓库信息
	fmt.Printf("🌟 Creating Lynx service %s, layout repo is %s, please wait a moment.\n\n", p.Name, layout)
	// 创建一个新的仓库实例
	repo := base.NewRepo(layout, branch)
	// 将远程仓库内容复制到目标路径，并排除 .git 和 .github 目录
	if err := repo.CopyTo(ctx, to, p.Name, []string{".git", ".github"}); err != nil {
		return err
	}
	// 重命名 cmd 目录下的 user 目录为项目名称
	e := os.Rename(
		filepath.Join(to, "cmd", "user"),
		filepath.Join(to, "cmd", p.Name),
	)
	if e != nil {
		return e
	}
	// 打印项目目录结构
	base.Tree(to, dir)

	// 提示用户项目创建成功
	fmt.Printf("\n🎉 Project creation succeeded %s\n", color.GreenString(p.Name))
	// 提示用户使用以下命令启动项目
	fmt.Print("💻 Use the following command to start the project 👇:\n\n")
	fmt.Println(color.WhiteString("$ cd %s", p.Name))
	fmt.Println(color.WhiteString("$ go generate ./..."))
	fmt.Println(color.WhiteString("$ go build -o ./bin/ ./... "))
	fmt.Println(color.WhiteString("$ ./bin/%s -conf ./configs\n", p.Name))
	// 感谢用户使用 Lynx 并提供教程链接
	fmt.Println("🤝 Thanks for using Lynx")
	fmt.Println("📚 Tutorial: https://go-lynx.cn/docs/start")
	return nil
}
