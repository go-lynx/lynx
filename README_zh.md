<p align="center"><a href="https://go-lynx.cn/" target="_blank"><img width="120" src="https://avatars.githubusercontent.com/u/150900434?s=250&u=8f8e9a5d1fab6f321b4aa350283197fc1d100efa&v=4" alt="logo"></a></p>

<p align="center">
<a href="https://pkg.go.dev/github.com/go-lynx/lynx"><img src="https://pkg.go.dev/badge/github.com/go-lynx/lynx/v2" alt="GoDoc"></a>
<a href="https://codecov.io/gh/go-lynx/lynx"><img src="https://codecov.io/gh/go-lynx/lynx/master/graph/badge.svg" alt="codeCov"></a>
<a href="https://goreportcard.com/report/github.com/go-lynx/lynx"><img src="https://goreportcard.com/badge/github.com/go-lynx/lynx" alt="Go Report Card"></a>
<a href="https://github.com/go-lynx/lynx/blob/main/LICENSE"><img src="https://img.shields.io/github/license/go-lynx/lynx" alt="License"></a>
<a href="https://discord.gg/2vq2Zsqq"><img src="https://img.shields.io/discord/1174545542689337497?label=chat&logo=discord" alt="Discord"></a>
</p>

Translations: [English](README.md) | [简体中文](README_zh.md)


## Lynx：即插即用的 Go 微服务框架

> Lynx 是一款革命性的开源微服务框架，为开发者提供无缝的即插即用体验。Lynx 建立在 Polaris 和 Kratos 的坚实基础之上，其主要目标是简化微服务的开发过程。它让开发者可以将精力集中在编写业务逻辑上，而不是陷入微服务基础设施的复杂性中。

## 主要特性

> Lynx 配备了一套综合的微服务关键能力，包括：

- **服务注册与发现：** 简化了在架构中定位和调用服务的过程，增强了系统的互操作性。
- **加密的内网通信：** 保证了你在微服务架构中数据的安全，培育了信任和可靠性。
- **限流：** 防范服务过载，确保一致和高质量的用户体验。
- **路由：** 促进了你的系统中有效的请求定向和流量管理，优化了性能。
- **降级：** 提供了优雅的故障处理，确保服务的可用性和弹性。
- **分布式事务：** 简化了跨多个服务的事务管理，促进了数据的一致性和可靠性。

## 插件驱动的模块化设计

> Lynx 自豪地推出了插件驱动的模块化设计，通过插件实现微服务功能模块的组合。这种独特的方法允许高度定制化和适应多样化的业务需求。任何第三方工具都可以轻松地作为插件集成，为开发者提供一个灵活和可扩展的平台。Lynx 致力于简化微服务生态系统，为开发者提供一个高效和用户友好的平台。

## 构建所用

Lynx 利用了几个开源项目的力量作为其核心组件，包括：

- [Seata](https://github.com/seata/seata)
- [Kratos](https://github.com/go-kratos/kratos)
- [Polaris](https://github.com/polarismesh/polaris)
## 快速安装

> 如果你想使用这个 Lynx 微服务，你只需要执行以下命令安装 Lynx CLI 命令行工具，然后运行新命令自动初始化一个可运行的项目（new 命令可以支持多个项目名称）。

```shell
go install github.com/go-lynx/lynx/cmd/lynx@latest
```

```shell
lynx new demo1 demo2 demo3
```

## 快速开始代码

想要快速启动你的微服务，使用以下代码（一些功能可以根据你的配置文件插入或移出）：

```go
func main() {
    boot.LynxApplication(wireApp).Run()
}
```

和我们一起，用 Lynx，这个即插即用的 Go 微服务框架，简化微服务开发。

## 钉钉交流

<img width="400" src="https://github.com/go-lynx/lynx/assets/32378959/cfeacfb8-95d4-4b23-8299-a868502f1076" alt="Ding Talk">