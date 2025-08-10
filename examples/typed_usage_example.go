// Package examples demonstrates the usage of typed plugins in the Lynx framework.
// This example shows the benefits of the generic-based approach over reflection.
package examples

import (
	"fmt"
	"log"
	"time"

	httpPlugin "github.com/go-lynx/lynx/plugins/service/http"
)

// TypedPluginUsageExample 展示泛型插件使用示例
func TypedPluginUsageExample() {
	fmt.Println("=== Lynx 框架泛型化改造示例 ===")

	// 1. 展示类型安全的插件获取（无反射）
	demonstrateTypeSafePluginAccess()

	// 2. 展示性能配置的类型安全应用
	demonstrateTypeSafeConfiguration()

	// 3. 展示编译时类型检查的好处
	demonstrateCompileTimeTypeChecking()

	// 4. 对比新旧方法的差异
	compareOldVsNewApproach()
}

// demonstrateTypeSafePluginAccess 展示类型安全的插件访问
func demonstrateTypeSafePluginAccess() {
	fmt.Println("\n1. 类型安全的插件访问:")

	// ✅ 新方法：类型安全，无反射，编译时检查
	fmt.Println("✅ 新方法（泛型）:")

	// 获取类型安全的 HTTP 服务器
	server, err := httpPlugin.GetTypedHTTPServer()
	if err != nil {
		fmt.Printf("   获取 HTTP 服务器失败: %v\n", err)
		fmt.Println("   注意：这在编译时就能发现问题，而不是运行时panic")
	} else {
		fmt.Printf("   ✓ 成功获取类型安全的 HTTP 服务器: %T\n", server)
	}

	// 获取完整的插件实例
	plugin, err := httpPlugin.GetHTTPPlugin()
	if err != nil {
		fmt.Printf("   获取 HTTP 插件失败: %v\n", err)
	} else {
		fmt.Printf("   ✓ 成功获取插件实例: %T\n", plugin)
		fmt.Printf("   ✓ 插件名称: %s\n", plugin.Name())
		fmt.Printf("   ✓ 插件版本: %s\n", plugin.Version())
	}
}

// demonstrateTypeSafeConfiguration 展示类型安全的配置应用
func demonstrateTypeSafeConfiguration() {
	fmt.Println("\n2. 类型安全的配置应用:")

	// ✅ 新方法：强类型配置，无反射
	fmt.Println("✅ 新方法（强类型配置）:")

	// 创建性能配置
	perfConfig := httpPlugin.HTTPPerformanceConfig{
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		MaxRequestSize:    1024 * 1024, // 1MB
	}

	// 应用配置（类型安全，无反射）
	err := httpPlugin.ConfigureHTTPPerformance(perfConfig)
	if err != nil {
		fmt.Printf("   配置应用失败: %v\n", err)
	} else {
		fmt.Println("   ✓ 成功应用性能配置")
		fmt.Printf("   ✓ 空闲超时: %v\n", perfConfig.IdleTimeout)
		fmt.Printf("   ✓ 读取头超时: %v\n", perfConfig.ReadHeaderTimeout)
		fmt.Printf("   ✓ 最大请求大小: %d 字节\n", perfConfig.MaxRequestSize)
	}
}

// demonstrateCompileTimeTypeChecking 展示编译时类型检查的好处
func demonstrateCompileTimeTypeChecking() {
	fmt.Println("\n3. 编译时类型检查的好处:")

	fmt.Println("✅ 新方法的优势:")
	fmt.Println("   • 编译时类型检查 - 错误在编译期发现")
	fmt.Println("   • IDE 智能提示 - 完整的代码补全")
	fmt.Println("   • 重构安全 - 类型变更会被编译器捕获")
	fmt.Println("   • 性能优化 - 无运行时反射开销")
	fmt.Println("   • 代码可读性 - 类型信息明确可见")

	// 示例：类型安全的方法调用
	plugin, err := httpPlugin.GetHTTPPlugin()
	if err == nil && plugin != nil {
		// 这些方法调用在编译时就能验证正确性
		server := plugin.GetHTTPServer()
		fmt.Printf("   ✓ 类型明确的服务器实例: %T\n", server)

		// 健康检查也是类型安全的
		if healthErr := plugin.CheckHealth(); healthErr != nil {
			fmt.Printf("   健康检查: %v\n", healthErr)
		} else {
			fmt.Println("   ✓ 插件健康状态良好")
		}
	}
}

// compareOldVsNewApproach 对比新旧方法的差异
func compareOldVsNewApproach() {
	fmt.Println("\n4. 新旧方法对比:")

	fmt.Println("❌ 旧方法（基于反射）的问题:")
	fmt.Println("   • 运行时类型断言可能 panic")
	fmt.Println("   • 无编译时类型检查")
	fmt.Println("   • IDE 无法提供智能提示")
	fmt.Println("   • 重构困难，容易出错")
	fmt.Println("   • 性能开销：每次调用都需要反射")

	fmt.Println("\n   旧方法示例代码:")
	fmt.Println(`   // ❌ 危险的类型断言
   plugin := app.Lynx().GetPluginManager().GetPlugin("http").(*ServiceHttp)
   server := plugin.server  // 可能 panic`)

	fmt.Println("\n✅ 新方法（泛型）的优势:")
	fmt.Println("   • 编译时类型安全")
	fmt.Println("   • 零反射开销")
	fmt.Println("   • 完整的 IDE 支持")
	fmt.Println("   • 重构友好")
	fmt.Println("   • 代码自文档化")

	fmt.Println("\n   新方法示例代码:")
	fmt.Println(`   // ✅ 类型安全的获取
   server, err := httpPlugin.GetTypedHTTPServer()
   if err != nil {
       // 优雅的错误处理
       return err
   }
   // 编译时保证 server 是 *http.Server 类型`)
}

// BenchmarkExample 性能对比示例（伪代码）
func BenchmarkExample() {
	fmt.Println("\n=== 性能对比 ===")

	fmt.Println("基准测试结果（模拟）:")
	fmt.Println("旧方法（反射）:    1000000 次操作    1500 ns/op")
	fmt.Println("新方法（泛型）:    1000000 次操作     100 ns/op")
	fmt.Println("性能提升: ~15x")

	fmt.Println("\n内存分配对比:")
	fmt.Println("旧方法: 每次调用分配 2-3 个对象")
	fmt.Println("新方法: 零额外内存分配")
}

// ErrorHandlingExample 错误处理示例
func ErrorHandlingExample() {
	fmt.Println("\n=== 错误处理对比 ===")

	fmt.Println("旧方法的问题:")
	fmt.Println("• 类型断言失败导致 panic")
	fmt.Println("• 运行时才能发现问题")
	fmt.Println("• 调试困难")

	fmt.Println("\n新方法的优势:")
	fmt.Println("• 返回明确的错误信息")
	fmt.Println("• 编译时发现类型问题")
	fmt.Println("• 优雅的错误处理")

	// 示例：安全的插件获取
	if httpPlugin.IsHTTPServerReady() {
		fmt.Println("✓ HTTP 服务器已就绪")
	} else {
		fmt.Println("⚠ HTTP 服务器未就绪")
	}

	// 获取服务器地址
	if addr, err := httpPlugin.GetHTTPServerAddr(); err == nil {
		fmt.Printf("✓ 服务器地址: %s\n", addr)
	} else {
		fmt.Printf("⚠ 获取地址失败: %v\n", err)
	}
}

// init 初始化示例
func init() {
	// 这个示例在实际运行时需要先初始化 Lynx 应用
	log.Println("Lynx 泛型化改造示例已加载")
	log.Println("使用 TypedPluginUsageExample() 查看完整示例")
}
