package base

import (
	"fmt"
	"os"
	"strings"
)

// Lang 返回当前语言代码，默认 zh。
func Lang() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LYNX_LANG")))
	if v == "en" {
		return "en"
	}
	return "zh"
}

// IsZH 表示当前是否中文环境。
func IsZH() bool { return Lang() != "en" }

// Choose 根据当前语言返回对应文案。
func Choose(zhMsg, enMsg string) string {
	if IsZH() {
		return zhMsg
	}
	return enMsg
}

// 内置消息字典，按语言存储
var messages = map[string]map[string]string{
	"zh": {
		"project_names":     "项目名称是什么？",
		"project_names_help": "使用空格分隔多个项目名称",
		"override_confirm":   "📂 是否覆盖已存在的目录？",
		"override_help":      "删除已存在目录并创建项目。",
		"already_exists":    "🚫 %s 已存在\n",
		"creating_service":  "🌟 正在创建 Lynx 服务 %s，模板仓库：%s，请稍等。\n\n",
		"mod_tidy_failed":   "\n⚠️  'go mod tidy' 执行失败: %v\n%s\n",
		"mod_tidy_ok":       "\n✅ 'go mod tidy' 完成\n",
		"project_success":   "\n🎉 项目创建成功 %s\n",
		"start_cmds_header": "💻 使用以下命令启动项目 👇:\n\n",
		"thanks":            "🤝 感谢使用 Lynx\n",
		"tutorial":          "📚 教程: https://go-lynx.cn/docs/start\n",
		"no_project_names":  "\n❌ 未找到项目名，请提供正确的项目名称\n",
		"failed_create":     "\x1b[31mERROR: 创建项目失败(%s)\x1b[m\n",
		"timeout":           "\x1b[31mERROR: 项目创建超时\x1b[m\n",
		// suggestions
		"suggestion_dns":        "   👉 建议：检查 DNS/网络，或配置代理（HTTP(S)_PROXY）；必要时更换镜像源",
		"suggestion_timeout":    "   👉 建议：检查网络波动与代理；可提高重试次数 LYNX_RETRIES、增大 LYNX_MAX_BACKOFF_MS",
		"suggestion_auth":       "   👉 建议：检查 git 凭据（ssh-agent/credential helper）；若使用 SSH，确保密钥与 known_hosts 正确",
		"suggestion_safe":       "   👉 建议：git config --global --add safe.directory <path>（在 CI/容器场景常见）",
		"suggestion_remote":     "   👉 建议：确认远端分支/标签是否存在；可改用 --ref 指定 tag 或 commit",
	},
	"en": {
		"project_names":     "What are the project names ?",
		"project_names_help": "Enter project names separated by space.",
		"override_confirm":   "📂 Do you want to override the folder ?",
		"override_help":      "Delete the existing folder and create the project.",
		"already_exists":    "🚫 %s already exists\n",
		"creating_service":  "🌟 Creating Lynx service %s, layout repo is %s, please wait a moment.\n\n",
		"mod_tidy_failed":   "\n⚠️  'go mod tidy' failed: %v\n%s\n",
		"mod_tidy_ok":       "\n✅ 'go mod tidy' completed\n",
		"project_success":   "\n🎉 Project creation succeeded %s\n",
		"start_cmds_header": "💻 Use the following command to start the project 👇:\n\n",
		"thanks":            "🤝 Thanks for using Lynx\n",
		"tutorial":          "📚 Tutorial: https://go-lynx.cn/docs/start\n",
		"no_project_names":  "\n❌ No project names found,Please provide the correct project name\n",
		"failed_create":     "\x1b[31mERROR: Failed to create project(%s)\x1b[m\n",
		"timeout":           "\x1b[31mERROR: project creation timed out\x1b[m\n",
		// suggestions
		"suggestion_dns":        "   👉 Suggestion: Check DNS/network, or set HTTP(S)_PROXY; switch to a mirror if needed",
		"suggestion_timeout":    "   👉 Suggestion: Check network/Proxy; increase LYNX_RETRIES and LYNX_MAX_BACKOFF_MS",
		"suggestion_auth":       "   👉 Suggestion: Verify Git credentials (ssh-agent/credential helper); for SSH, ensure keys and known_hosts are correct",
		"suggestion_safe":       "   👉 Suggestion: git config --global --add safe.directory <path> (common in CI/containers)",
		"suggestion_remote":     "   👉 Suggestion: Ensure remote branch/tag exists; use --ref to specify a tag or commit",
	},
}

// T 根据 key 与当前语言返回格式化后的消息
func T(key string, args ...any) string {
	lang := Lang()
	if dict, ok := messages[lang]; ok {
		if tmpl, ok2 := dict[key]; ok2 {
			return fmt.Sprintf(tmpl, args...)
		}
	}
	// fallback: 返回 key 本身或简单拼接
	if len(args) == 0 {
		return key
	}
	return fmt.Sprintf(key, args...)
}
