<p align="center"><a href="https://go-lynx.cn/" target="_blank"><img width="120" src="https://avatars.githubusercontent.com/u/150900434?s=250&u=8f8e9a5d1fab6f321b4aa350283197fc1d100efa&v=4" alt="logo"></a></p>

<p align="center">
<a href="https://pkg.go.dev/github.com/go-lynx/lynx"><img src="https://pkg.go.dev/badge/github.com/go-lynx/lynx/v2" alt="GoDoc"></a>
<a href="https://codecov.io/gh/go-lynx/lynx"><img src="https://codecov.io/gh/go-lynx/lynx/master/graph/badge.svg" alt="codeCov"></a>
<a href="https://goreportcard.com/report/github.com/go-lynx/lynx"><img src="https://goreportcard.com/badge/github.com/go-lynx/lynx" alt="Go Report Card"></a>
<a href="https://github.com/go-lynx/lynx/blob/main/LICENSE"><img src="https://img.shields.io/github/license/go-lynx/lynx" alt="License"></a>
<a href="https://discord.gg/2vq2Zsqq"><img src="https://img.shields.io/discord/1174545542689337497?label=chat&logo=discord" alt="Discord"></a>
</p>


## Lynx: The Plug-and-Play Go Microservices Framework

> Lynx is a revolutionary open-source microservices framework, offering a seamless plug-and-play experience for developers. Built on the robust foundations of Polaris and Kratos, Lynx's primary objective is to simplify the microservices development process. It allows developers to focus their efforts on crafting business logic, rather than getting entangled in the complexities of microservice infrastructure.

## Key Features

> Lynx comes equipped with a comprehensive suite of essential microservices capabilities, including:

- **Service Registration & Discovery:** Streamlines the process of locating and invoking services across your architecture, enhancing system interoperability.
- **Encrypted Intra-Network Communication:** Guarantees the security of your data within your microservices architecture, fostering trust and reliability.
- **Rate Limiting:** Safeguards against service overloads, ensuring a consistent and high-quality user experience.
- **Routing:** Facilitates efficient request direction and traffic management within your system, optimizing performance.
- **Degradation:** Provides graceful failure handling, ensuring service availability and resilience.
- **Distributed Transactions:** Simplifies the management of transactions across multiple services, promoting data consistency and reliability.

## Plugin-Driven Modular Design

> Lynx proudly introduces a plugin-driven modular design, enabling the combination of microservice functionality modules through plugins. This unique approach allows for high customizability and adaptability to diverse business needs. Any third-party tool can be effortlessly integrated as a plugin, providing a flexible and extensible platform for developers. Lynx is committed to simplifying the microservices ecosystem, delivering an efficient and user-friendly platform for developers.

## Built With

Lynx harnesses the power of several open-source projects for its core components, including:

- [Seata](https://github.com/seata/seata)
- [Kratos](https://github.com/go-kratos/kratos)
- [Polaris](https://github.com/polarismesh/polaris)
## Quick Install

> If you want to use this lynx microservice, all you need to do is execute the following command to install the Lynx CLI command-line tool, and then run the new command to automatically initialize a runnable project (the new command can support multiple project names).

```shell
go install github.com/go-lynx/lynx/cmd/lynx@latest
```

```shell
lynx new demo1 demo2 demo3
```

## Quick Start Code

To get your microservice up and running in no time, use the following code (Some functionalities can be plugged in or out based on your choice.):

```go
func main() {
    boot.LynxApplication(wireApp).Run()
}
```

Join us in our journey to simplify microservices development with Lynx, the plug-and-play Go Microservices Framework.

## Ding Talk

<img width="500" src="https://github.com/go-lynx/lynx/assets/32378959/cfeacfb8-95d4-4b23-8299-a868502f1076" alt="Ding Talk">

