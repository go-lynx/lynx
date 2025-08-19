package main

import (
	"log"
	"os"

	"github.com/go-lynx/lynx/cmd/lynx/internal/project"
	"github.com/spf13/cobra"
)

// rootCmd 是 Lynx 命令行工具的根命令，定义了工具的基本信息和版本。
var rootCmd = &cobra.Command{
	// Use 定义了命令的使用方式
	Use: "lynx",
	// Short 是命令的简短描述
	Short: "Lynx: The Plug-and-Play Go Microservices Framework",
	// Long 是命令的详细描述
	Long: `Lynx: The Plug-and-Play Go Microservices Framework`,
	// Version 定义了命令行工具的版本，release 变量需在别处定义
	Version: release,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 将日志级别持久化到环境变量，供内部子命令与执行器读取
		verbose, _ := cmd.Flags().GetBool("verbose")
		quiet, _ := cmd.Flags().GetBool("quiet")
		logLevel, _ := cmd.Flags().GetString("log-level")
		lang, _ := cmd.Flags().GetString("lang")

		// 语言环境
		if lang != "" {
			_ = os.Setenv("LYNX_LANG", lang)
		}

		// 日志级别优先级：--log-level > --quiet/--verbose > 默认
		if logLevel != "" {
			_ = os.Setenv("LYNX_LOG_LEVEL", logLevel)
		} else if quiet {
			_ = os.Setenv("LYNX_LOG_LEVEL", "error")
			_ = os.Setenv("LYNX_QUIET", "1")
		} else if verbose {
			_ = os.Setenv("LYNX_LOG_LEVEL", "debug")
			_ = os.Setenv("LYNX_VERBOSE", "1")
		} else {
			// 默认 info
			if os.Getenv("LYNX_LOG_LEVEL") == "" {
				_ = os.Setenv("LYNX_LOG_LEVEL", "info")
			}
		}
	},
}

// init 函数是包的初始化函数，在包被加载时自动执行。
func init() {
	// 为根命令添加子命令，这里添加了 project 包中的 CmdNew 命令
	rootCmd.AddCommand(project.CmdNew)
	// 全局日志级别标志
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose logs")
	rootCmd.PersistentFlags().Bool("quiet", false, "suppress non-error logs")
	rootCmd.PersistentFlags().String("log-level", "info", "log level: error|warn|info|debug (overrides --quiet/--verbose)")
	rootCmd.PersistentFlags().String("lang", "zh", "language for messages: zh|en")
}

// main 函数是程序的入口点，负责执行根命令。
func main() {
	// 执行根命令，如果执行过程中出现错误则记录错误日志并终止程序
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
