package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/go-lynx/lynx/cmd/lynx/internal/base"
	"github.com/spf13/cobra"
)

// CmdNew 代表 `new` 命令，用于创建 Lynx 服务模板项目。
var CmdNew = &cobra.Command{
	Use:   "new",
	Short: "Create a lynx service template",
	Long:  "Create a lynx service project using the repository template.",
	RunE:  run, // 执行命令时调用的函数
}

var (
	// repoURL 存储布局仓库的 URL。
	repoURL string
	// branch 存储仓库的分支名称。
	branch string
	// ref 存储仓库的引用（优先于 branch）。
	ref string
	// timeout 存储项目创建操作的超时时间。
	timeout string
	// module 存储生成项目的 Go module 路径。
	module string
	// force 存储是否强制覆盖已存在目录的标志。
	force bool
	// postTidy 存储是否在创建后执行 go mod tidy 的标志。
	postTidy bool
	// concurrency 存储并发上限。
	concurrency int
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
	// 为命令添加 --ref 标志，统一指定 commit/tag/branch（优先生效）
	CmdNew.Flags().StringVar(&ref, "ref", ref, "repo ref (commit/tag/branch), takes precedence over --branch")
	// 为命令添加 --timeout 标志，用于指定超时时间
	CmdNew.Flags().StringVarP(&timeout, "timeout", "t", timeout, "time out")
	// 为命令添加 --module 标志，指定生成项目的 Go module 路径（例如 github.com/acme/foo）
	CmdNew.Flags().StringVarP(&module, "module", "m", module, "go module path for the new project")
	// 为命令添加 --force 标志，非交互覆盖已存在目录
	CmdNew.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing directory without prompt")
	// 为命令添加 --post-tidy 标志，创建后自动执行 go mod tidy（默认关闭，仅提示）
	CmdNew.Flags().BoolVar(&postTidy, "post-tidy", false, "run 'go mod tidy' in the new project after creation")
	// 为命令添加 --concurrency 并发上限（默认 min(4, NumCPU*2)）
	defaultConc := runtime.NumCPU() * 2
	if defaultConc > 4 {
		defaultConc = 4
	}
	if defaultConc < 1 {
		defaultConc = 1
	}
	concurrency = defaultConc
	CmdNew.Flags().IntVarP(&concurrency, "concurrency", "c", concurrency, "max concurrent project creations")
}

// run 是 `new` 命令的执行函数，负责创建 Lynx 服务项目。
func run(_ *cobra.Command, args []string) error {
	// 获取当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 将超时时间字符串解析为 time.Duration 类型
	t, err := time.ParseDuration(timeout)
	if err != nil {
		return err
	}

	// 创建带有超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel() // 确保在函数结束时取消上下文

	var names []string
	if len(args) == 0 {
		// 若未通过命令行参数提供项目名称，则提示用户输入
		prompt := &survey.Input{
			Message: base.T("project_names"),
			Help:    base.T("project_names_help"),
		}
		var input string
		errAsk := survey.AskOne(prompt, &input)
		if errAsk != nil || input == "" {
			base.Errorf("%s", base.T("no_project_names"))
			return fmt.Errorf("no project names provided")
		}
		// 将用户输入的项目名称按空格分割
		names = strings.Split(input, " ")
	} else {
		names = args
	}

	// 检查并去除重复的项目名称
	names = checkDuplicates(names)
	if len(names) < 1 {
		base.Errorf("%s", base.T("no_project_names"))
		return fmt.Errorf("no valid project names after de-duplication")
	}

	// 并发创建多个项目（增加并发上限）
	done := make(chan error, len(names))
	var wg sync.WaitGroup
	// 运行时并发上限优先使用 --concurrency，其次按 CPU * 2
	maxConc := concurrency
	if maxConc <= 0 {
		maxConc = runtime.NumCPU() * 2
	}
	if maxConc < 1 {
		maxConc = 1
	}
	if len(names) < maxConc {
		maxConc = len(names)
	}
	sem := make(chan struct{}, maxConc)
	// 计算实际引用：--ref 优先，其次 --branch
	effectiveRef := ref
	if strings.TrimSpace(effectiveRef) == "" {
		effectiveRef = branch
	}
	for _, name := range names {
		// 处理项目名称和工作目录参数
		projectName, workingDir := processProjectParams(name, wd)
		p := &Project{Name: projectName}
		wg.Add(1)
		sem <- struct{}{}
		go func(p *Project, workingDir string) {
			// 调用 Project 的 New 方法创建项目，并将结果发送到 done 通道
			defer func() { <-sem; wg.Done() }()
			done <- p.New(ctx, workingDir, repoURL, effectiveRef, force, module, postTidy)
		}(p, workingDir)
	}

	wg.Wait()   // 等待所有 goroutine 完成
	close(done) // 关闭 done 通道

	// 从 done 通道读取错误信息并打印
	fail := 0
	for err := range done {
		if err != nil {
			fail++
			base.Errorf("%s", fmt.Sprintf(base.T("failed_create"), err.Error()))
			// 提示可操作建议
			printSuggestions(err.Error())
		}
	}
	// 检查上下文是否因超时而取消
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		base.Errorf("%s", base.T("timeout"))
		return context.DeadlineExceeded
	}
	if fail > 0 {
		return fmt.Errorf("%d project(s) failed", fail)
	}
	return nil
}

// 根据错误关键信息打印可操作建议
func printSuggestions(msg string) {
	low := strings.ToLower(msg)
	say := func(key string, args ...any) { fmt.Fprintln(os.Stderr, fmt.Sprintf(base.T(key), args...)) }
	switch {
	case strings.Contains(low, "could not resolve host") || strings.Contains(low, "couldn't resolve host") || strings.Contains(low, "name or service not known"):
		say("suggestion_dns")
	case strings.Contains(low, "timed out") || strings.Contains(low, "timeout") || strings.Contains(low, "i/o timeout"):
		say("suggestion_timeout")
	case strings.Contains(low, "authentication") || strings.Contains(low, "permission denied") || strings.Contains(low, "auth"):
		say("suggestion_auth")
	case strings.Contains(low, "safe.directory"):
		say("suggestion_safe")
	case strings.Contains(low, "not found") && strings.Contains(low, "origin/"):
		say("suggestion_remote")
	}
}

// processProjectParams 处理项目名称参数，返回处理后的项目名称和工作目录。
func processProjectParams(projectName string, workingDir string) (projectNameResult, workingDirResult string) {
	_projectDir := projectName
	// 仅在以 "~/" 开头时展开家目录
	if strings.HasPrefix(projectName, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			_projectDir = filepath.Join(homeDir, projectName[2:])
		}
	}

	// 检查项目名称是否为相对路径，若是则转换为绝对路径
	if !filepath.IsAbs(_projectDir) {
		joined := filepath.Join(workingDir, _projectDir)
		if absPath, err := filepath.Abs(joined); err == nil {
			_projectDir = absPath
		} else {
			// 回退：使用拼接路径
			_projectDir = joined
		}
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
