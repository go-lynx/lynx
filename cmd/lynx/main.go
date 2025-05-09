package main

import (
	"github.com/go-lynx/lynx/cmd/lynx/internal/project"
	"github.com/spf13/cobra"
	"log"
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
}

// init 函数是包的初始化函数，在包被加载时自动执行。
func init() {
	// 为根命令添加子命令，这里添加了 project 包中的 CmdNew 命令
	rootCmd.AddCommand(project.CmdNew)
}

// main 函数是程序的入口点，负责执行根命令。
func main() {
	// 执行根命令，如果执行过程中出现错误则记录错误日志并终止程序
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
