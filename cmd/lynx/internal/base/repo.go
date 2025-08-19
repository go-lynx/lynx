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
	// 确保 git 可用
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH: %w", err)
	}
	// 确保目录存在
	if _, err := os.Stat(r.Path()); os.IsNotExist(err) {
		return fmt.Errorf("repo path does not exist: %s", r.Path())
	}
	// fetch 所有远端与 tags（带重试），并 prune 过期引用
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"fetch", "--all", "--tags", "--prune"}, 2); err != nil {
		return fmt.Errorf("git fetch failed (check network/proxy or git auth): %w", err)
	}
	// 目标 ref：优先 r.branch（其实是 ref），为空则维持当前 HEAD
	ref := strings.TrimSpace(r.branch)
	if ref == "" {
		// 无特定 ref，保持当前分支并与远端同步（若在分离头指针则跳过）
		if bOut, bErr := RunCMD(ctx, r.Path(), "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, 0); bErr == nil {
			cur := strings.TrimSpace(bOut)
			if cur != "HEAD" && cur != "" {
				if _, err := RunCMD(ctx, r.Path(), "git", []string{"reset", "--hard", fmt.Sprintf("origin/%s", cur)}, 1); err != nil {
					return fmt.Errorf("git reset --hard origin/%s failed: %w", cur, err)
				}
			}
		}
		return nil
	}
	// 有 ref：优先当作远程分支，否则当作 tag/commit（分离头指针）
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"show-ref", "--verify", fmt.Sprintf("refs/remotes/origin/%s", ref)}, 0); err == nil {
		// 远程分支存在：切到该分支并强制同步远端
		if _, err := RunCMD(ctx, r.Path(), "git", []string{"checkout", "-B", ref, fmt.Sprintf("origin/%s", ref)}, 0); err != nil {
			return fmt.Errorf("git checkout branch %s failed: %w", ref, err)
		}
		if _, err := RunCMD(ctx, r.Path(), "git", []string{"reset", "--hard", fmt.Sprintf("origin/%s", ref)}, 1); err != nil {
			return fmt.Errorf("git reset --hard origin/%s failed: %w", ref, err)
		}
		return nil
	}
	// 视为 tag/commit：分离头指针检出
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"checkout", "--detach", ref}, 0); err != nil {
		return fmt.Errorf("git checkout %s failed (tag/commit?): %w", ref, err)
	}
	return nil
}

// Clone 将远程仓库克隆到本地缓存路径。如果本地缓存路径已存在，则尝试拉取最新代码。
// 参数 ctx 是上下文，用于控制操作的生命周期。
// 返回值为操作过程中可能出现的错误。
func (r *Repo) Clone(ctx context.Context) error {
	// 确保 git 可用
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH: %w", err)
	}
	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(r.Path()), 0o755); err != nil {
		return fmt.Errorf("prepare repo parent dir failed: %w", err)
	}
	// 检查本地缓存路径是否已存在
	if _, err := os.Stat(r.Path()); !os.IsNotExist(err) {
		// 若存在，尝试拉取最新代码
		return r.Pull(ctx)
	}
	var err error
	// 优先浅克隆默认分支，之后再切换到 ref（更通用）
	_, err = RunCMD(ctx, filepath.Dir(r.Path()), "git", []string{"clone", "--depth", "1", r.url, r.Path()}, 2)
	if err != nil { return fmt.Errorf("git clone failed: %w", err) }
	// 获取 tags 与所有远端引用
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"fetch", "--all", "--tags", "--prune"}, 2); err != nil {
		return fmt.Errorf("git fetch after clone failed: %w", err)
	}
	// 若指定 ref，则尝试切换
	ref := strings.TrimSpace(r.branch)
	if ref == "" {
		return nil
	}
	// 优先作为远程分支
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"show-ref", "--verify", fmt.Sprintf("refs/remotes/origin/%s", ref)}, 0); err == nil {
		if _, err := RunCMD(ctx, r.Path(), "git", []string{"checkout", "-B", ref, fmt.Sprintf("origin/%s", ref)}, 0); err != nil {
			return fmt.Errorf("git checkout branch %s failed: %w", ref, err)
		}
		return nil
	}
	// 作为 tag/commit 分离头指针
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"checkout", "--detach", ref}, 0); err != nil {
		return fmt.Errorf("git checkout %s failed (tag/commit?): %w", ref, err)
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
	// 当提供了 modPath 时，才将模块路径替换加入列表；若为空则不替换模板 module
	if strings.TrimSpace(modPath) != "" {
		replaces = append([]string{mod, modPath}, replaces...)
	}
	// 复制目录内容
	return copyDir(r.Path(), to, replaces, ignores)
}
