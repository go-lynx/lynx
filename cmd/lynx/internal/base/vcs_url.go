package base

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// scpSyntaxRe 用于匹配 SCP 风格的 URL 语法，例如 "git@github.com:user/repo"。
var (
	scpSyntaxRe = regexp.MustCompile(`^(\w+)@([\w.-]+):(.*)$`)
	// scheme 定义了支持的版本控制系统（VCS）URL 协议列表。
	scheme = []string{"git", "https", "http", "git+ssh", "ssh", "file", "ftp", "ftps"}
)

// ParseVCSUrl 解析版本控制系统（VCS）的仓库 URL。
// 参考 https://github.com/golang/go/blob/master/src/cmd/go/internal/vcs/vcs.go
// 查看 https://go-review.googlesource.com/c/go/+/12226/
// Git URL 定义参考 https://git-scm.com/docs/git-clone#_git_urls
// 参数 repo 是需要解析的仓库 URL 字符串。
// 返回值为解析后的 *url.URL 实例和可能出现的错误。
func ParseVCSUrl(repo string) (*url.URL, error) {
	var (
		repoURL *url.URL
		err     error
	)

	// 检查是否为 SCP 风格的 URL 语法
	if m := scpSyntaxRe.FindStringSubmatch(repo); m != nil {
		// 若匹配 SCP 风格的语法，则将其转换为标准 URL 格式。
		// 例如，"git@github.com:user/repo" 会转换为 "ssh://git@github.com/user/repo"。
		repoURL = &url.URL{
			Scheme: "ssh",
			User:   url.User(m[1]),
			Host:   m[2],
			Path:   m[3],
		}
	} else {
		// 若不是 SCP 风格的语法，确保 URL 包含 "//"
		if !strings.Contains(repo, "//") {
			repo = "//" + repo
		}
		// 处理以 "//git@" 开头的 URL，将其转换为 "ssh:" 协议
		if strings.HasPrefix(repo, "//git@") {
			repo = "ssh:" + repo
		} else if strings.HasPrefix(repo, "//") {
			// 处理以 "//" 开头的 URL，将其转换为 "https:" 协议
			repo = "https:" + repo
		}
		// 使用标准库的 url.Parse 函数解析 URL
		repoURL, err = url.Parse(repo)
		if err != nil {
			return nil, err
		}
	}

	// 检查解析后的 URL 协议是否在支持的协议列表中
	// 同时也会检查不安全的协议，因为此函数仅用于报告仓库 URL 的状态
	for _, s := range scheme {
		if repoURL.Scheme == s {
			return repoURL, nil
		}
	}
	// 若协议不支持，则返回错误
	return nil, errors.New("unable to parse repo url")
}
