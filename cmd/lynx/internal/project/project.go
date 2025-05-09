package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// CmdNew 代表 `new` 命令，用于创建 Lynx 服务模板项目。
var CmdNew = &cobra.Command{
	Use:   "new",
	Short: "Create a lynx service template",
	Long:  "Create a lynx service project using the repository template.",
	Run:   run, // 执行命令时调用的函数
}

// repoURL 存储布局仓库的 URL。
// branch 存储仓库的分支名称。
// timeout 存储项目创建操作的超时时间。
var (
	repoURL string
	branch  string
	timeout string
)

// init 是包的初始化函数，用于设置默认值和命令行标志。
func init() {
	// 从环境变量 LYNX_LAYOUT_REPO 获取仓库 URL，若为空则使用默认值
	if repoURL = os.Getenv("LYNX_LAYOUT_REPO"); repoURL == "" {
		repoURL = "https://github.com/go-lynx/lynx-layout.git"
	}
	timeout = "60s" // 默认超时时间为 60 秒
	// 为命令添加 --repo-url 标志，用于指定布局仓库 URL
	CmdNew.Flags().StringVarP(&repoURL, "repo-url", "r", repoURL, "layout repo")
	// 为命令添加 --branch 标志，用于指定仓库分支
	CmdNew.Flags().StringVarP(&branch, "branch", "b", branch, "repo branch")
	// 为命令添加 --timeout 标志，用于指定超时时间
	CmdNew.Flags().StringVarP(&timeout, "timeout", "t", timeout, "time out")
}

// run 是 `new` 命令的执行函数，负责创建 Lynx 服务项目。
func run(_ *cobra.Command, args []string) {
	// 获取当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// 将超时时间字符串解析为 time.Duration 类型
	t, err := time.ParseDuration(timeout)
	if err != nil {
		panic(err)
	}

	// 创建带有超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel() // 确保在函数结束时取消上下文

	var names []string
	if len(args) == 0 {
		// 若未通过命令行参数提供项目名称，则提示用户输入
		prompt := &survey.Input{
			Message: "What are the project names ?",
			Help:    "Enter project names separated by space.",
		}
		var input string
		err = survey.AskOne(prompt, &input)
		if err != nil || input == "" {
			fmt.Printf("\n❌ No project names found,Please provide the correct project name\n")
			return
		}
		// 将用户输入的项目名称按空格分割
		names = strings.Split(input, " ")
	} else {
		names = args
	}

	// 检查并去除重复的项目名称
	names = checkDuplicates(names)
	if len(names) < 1 {
		fmt.Printf("\n❌ No project names found,Please provide the correct project name\n")
		return
	}

	// 并发创建多个项目
	done := make(chan error, len(names))
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		// 处理项目名称和工作目录参数
		projectName, workingDir := processProjectParams(name, wd)
		p := &Project{Name: projectName}
		go func() {
			// 调用 Project 的 New 方法创建项目，并将结果发送到 done 通道
			done <- p.New(ctx, workingDir, repoURL, branch)
			wg.Done()
		}()
	}

	wg.Wait()   // 等待所有 goroutine 完成
	close(done) // 关闭 done 通道

	// 从 done 通道读取错误信息并打印
	for err := range done {
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "\033[31mERROR: Failed to create project(%s)\033[m\n", err.Error())
		}
	}
	// 检查上下文是否因超时而取消
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		_, _ = fmt.Fprint(os.Stderr, "\033[31mERROR: project creation timed out\033[m\n")
		return
	}
}

// processProjectParams 处理项目名称参数，返回处理后的项目名称和工作目录。
func processProjectParams(projectName string, workingDir string) (projectNameResult, workingDirResult string) {
	_projectDir := projectName
	_workingDir := workingDir
	// 处理以 ~ 开头的项目名称，将其替换为用户主目录
	if strings.HasPrefix(projectName, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// 若无法获取用户主目录，则返回原始值
			return _projectDir, _workingDir
		}
		_projectDir = filepath.Join(homeDir, projectName[2:])
	}

	// 检查项目名称是否为相对路径，若是则转换为绝对路径
	if !filepath.IsAbs(projectName) {
		absPath, err := filepath.Abs(projectName)
		if err != nil {
			return _projectDir, _workingDir
		}
		_projectDir = absPath
	}

	// 返回处理后的项目名称（路径最后一部分）和工作目录（路径目录部分）
	return filepath.Base(_projectDir), filepath.Dir(_projectDir)
}

// checkDuplicates 检查并去除项目名称列表中的重复项，同时验证名称的合法性。
func checkDuplicates(names []string) []string {
	encountered := map[string]bool{}
	var result []string

	// 定义项目名称的合法字符模式
	pattern := `^[A-Za-z0-9_-]+$`
	regex := regexp.MustCompile(pattern)

	for _, name := range names {
		// 若名称符合模式且未出现过，则添加到结果列表
		if regex.MatchString(name) && !encountered[name] {
			encountered[name] = true
			result = append(result, name)
		}
	}
	return result
}
