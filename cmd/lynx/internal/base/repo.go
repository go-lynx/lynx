package base

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// unExpandVarPath 定义了不需要展开变量的路径前缀列表。
var unExpandVarPath = []string{"~", ".", ".."}

// Repo 表示 Git 仓库管理器，用于管理仓库的克隆、拉取和复制等操作。
type Repo struct {
	url    string // 仓库的远程 URL
	home   string // 仓库的本地缓存路径
	branch string // 要操作的仓库分支
}

// repoDir 根据仓库的 URL 生成一个相对唯一的目录名。
// 参数 url 是仓库的远程 URL。
// 返回值为生成的目录名。
func repoDir(url string) string {
	// 解析仓库的 VCS URL
	vcsURL, err := ParseVCSUrl(url)
	if err != nil {
		// 解析失败时直接返回原始 URL
		return url
	}
	// 检查主机名是否包含端口号
	host, _, err := net.SplitHostPort(vcsURL.Host)
	if err != nil {
		// 不包含端口号时，直接使用原始主机名
		host = vcsURL.Host
	}
	// 去除主机名中不需要展开变量的路径前缀
	for _, p := range unExpandVarPath {
		host = strings.TrimLeft(host, p)
	}
	// 获取 URL 路径中倒数第二个目录名
	dir := path.Base(path.Dir(vcsURL.Path))
	// 组合主机名和目录名作为最终的目录名
	url = fmt.Sprintf("%s/%s", host, dir)
	return url
}

// NewRepo 创建一个新的仓库管理器实例。
// 参数 url 是仓库的远程 URL，branch 是要操作的仓库分支。
// 返回值为新创建的 Repo 实例指针。
func NewRepo(url string, branch string) *Repo {
	return &Repo{
		url: url,
		// 计算仓库的本地缓存路径
		home:   lynxHomeWithDir("repo/" + repoDir(url)),
		branch: branch,
	}
}

// Path 返回仓库的本地缓存路径。
// 返回值为本地缓存路径的字符串。
func (r *Repo) Path() string {
	// 找到 URL 中最后一个 '/' 的索引
	start := strings.LastIndex(r.url, "/")
	// 找到 URL 中最后一个 '.git' 的索引
	end := strings.LastIndex(r.url, ".git")
	if end == -1 {
		// 若没有 '.git'，则取 URL 的长度
		end = len(r.url)
	}
	var branch string
	if r.branch == "" {
		// 若分支名为空，默认使用 '@main'
		branch = "@main"
	} else {
		// 否则，添加 '@' 前缀
		branch = "@" + r.branch
	}
	// 组合本地缓存路径
	return path.Join(r.home, r.url[start+1:end]+branch)
}

// Pull 从远程仓库拉取最新代码到本地缓存路径。
// 参数 ctx 是上下文，用于控制操作的生命周期。
// 返回值为操作过程中可能出现的错误。
func (r *Repo) Pull(ctx context.Context) error {
	// 检查本地仓库是否为有效的 Git 仓库
	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "HEAD")
	cmd.Dir = r.Path()
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	// 执行 git pull 命令拉取最新代码
	cmd = exec.CommandContext(ctx, "git", "pull")
	cmd.Dir = r.Path()
	out, err := cmd.CombinedOutput()
	// 打印命令执行输出
	fmt.Println(string(out))
	if err != nil {
		return err
	}
	return err
}

// Clone 将远程仓库克隆到本地缓存路径。如果本地缓存路径已存在，则尝试拉取最新代码。
// 参数 ctx 是上下文，用于控制操作的生命周期。
// 返回值为操作过程中可能出现的错误。
func (r *Repo) Clone(ctx context.Context) error {
	// 检查本地缓存路径是否已存在
	if _, err := os.Stat(r.Path()); !os.IsNotExist(err) {
		// 若存在，尝试拉取最新代码
		return r.Pull(ctx)
	}
	var cmd *exec.Cmd
	if r.branch == "" {
		// 若分支名为空，克隆默认分支
		cmd = exec.CommandContext(ctx, "git", "clone", r.url, r.Path())
	} else {
		// 否则，克隆指定分支
		cmd = exec.CommandContext(ctx, "git", "clone", "-b", r.branch, r.url, r.Path())
	}
	out, err := cmd.CombinedOutput()
	// 打印命令执行输出
	fmt.Println(string(out))
	if err != nil {
		return err
	}
	return nil
}

// CopyTo 将克隆后的仓库内容复制到指定的项目路径。
// 参数 ctx 是上下文，用于控制操作的生命周期；to 是目标项目路径；modPath 是模块路径；ignores 是需要忽略的文件或目录列表。
// 返回值为操作过程中可能出现的错误。
func (r *Repo) CopyTo(ctx context.Context, to string, modPath string, ignores []string) error {
	// 先克隆仓库到本地缓存路径
	if err := r.Clone(ctx); err != nil {
		return err
	}
	// 获取本地缓存路径下 go.mod 文件中的模块路径
	mod, err := ModulePath(filepath.Join(r.Path(), "go.mod"))
	if err != nil {
		return err
	}
	// 复制目录内容
	return copyDir(r.Path(), to, []string{mod, modPath}, ignores)
}

// CopyToV2 将克隆后的仓库内容复制到指定的项目路径，支持更多的替换规则。
// 参数 ctx 是上下文，用于控制操作的生命周期；to 是目标项目路径；modPath 是模块路径；ignores 是需要忽略的文件或目录列表；replaces 是需要替换的内容列表。
// 返回值为操作过程中可能出现的错误。
func (r *Repo) CopyToV2(ctx context.Context, to string, modPath string, ignores, replaces []string) error {
	// 先克隆仓库到本地缓存路径
	if err := r.Clone(ctx); err != nil {
		return err
	}
	// 获取本地缓存路径下 go.mod 文件中的模块路径
	mod, err := ModulePath(filepath.Join(r.Path(), "go.mod"))
	if err != nil {
		return err
	}
	// 将模块路径和 modPath 添加到替换列表
	replaces = append([]string{mod, modPath}, replaces...)
	// 复制目录内容
	return copyDir(r.Path(), to, replaces, ignores)
}
