package project

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
// force: 是否强制覆盖已存在的项目目录。
// module: 如果提供，则替换模板 go.mod 的 module。
// postTidy: 是否执行 go mod tidy 命令。
// 返回值: 若操作过程中出现错误，则返回相应的错误信息；否则返回 nil。
func (p *Project) New(ctx context.Context, dir string, layout string, branch string, force bool, module string, postTidy bool) error {
	// 计算项目最终创建的完整路径
	to := filepath.Join(dir, p.Name)

	// 检查目标路径是否已存在
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		// 若存在，提示用户路径已存在
		base.Warnf("%s", fmt.Sprintf(base.T("already_exists"), p.Name))
		// --force 则静默覆盖，否则交互确认
		if !force {
			prompt := &survey.Confirm{
				Message: base.T("override_confirm"),
				Help:    base.T("override_help"),
			}
			var override bool
			if e := survey.AskOne(prompt, &override); e != nil {
				return e
			}
			if !override {
				return err
			}
		}
		if e := os.RemoveAll(to); e != nil {
			return e
		}
	}

	// 提示用户开始创建项目，并显示项目名称和布局仓库信息
	base.Infof("%s", fmt.Sprintf(base.T("creating_service"), p.Name, layout))
	// 创建一个新的仓库实例
	repo := base.NewRepo(layout, branch)
	// 将远程仓库内容复制到目标路径，并排除 .git 和 .github 目录
	// 若提供 --module，则替换模板 go.mod 的 module；否则不替换模板 module
	if err := repo.CopyToV2(ctx, to, module, []string{".git", ".github"}, nil); err != nil {
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

	// 可选：执行 go mod tidy
	if postTidy {
		cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
		cmd.Dir = to
		if out, err := cmd.CombinedOutput(); err != nil {
			base.Warnf("%s", fmt.Sprintf(base.T("mod_tidy_failed"), err, string(out)))
		} else {
			base.Infof("%s", base.T("mod_tidy_ok"))
		}
	}

	// 提示用户项目创建成功
	base.Infof("%s", fmt.Sprintf(base.T("project_success"), color.GreenString(p.Name)))
	// 提示用户使用以下命令启动项目
	base.Infof("%s", base.T("start_cmds_header"))
	base.Infof("%s\n", color.WhiteString("$ cd %s", p.Name))
	if !postTidy {
		base.Infof("%s\n", color.WhiteString("$ go mod tidy"))
	}
	base.Infof("%s\n", color.WhiteString("$ go generate ./..."))
	base.Infof("%s\n", color.WhiteString("$ go build -o ./bin/ ./... "))
	base.Infof("%s\n", color.WhiteString("$ ./bin/%s -conf ./configs", p.Name))
	// 感谢用户使用 Lynx 并提供教程链接
	base.Infof("%s", base.T("thanks"))
	base.Infof("%s", base.T("tutorial"))
	return nil
}
