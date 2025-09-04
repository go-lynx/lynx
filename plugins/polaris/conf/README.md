# Polaris 配置文件说明

## 概述

Polaris 是腾讯开源的云原生服务发现和治理中心。本插件支持通过配置文件来配置 Polaris SDK 的连接参数。

## 配置文件结构

### 1. Lynx 应用配置文件

在你的应用配置文件中添加 Polaris 插件配置：

```yaml
lynx:
  polaris:
    namespace: "default"                    # 命名空间
    token: "your-polaris-token"            # 认证令牌（可选）
    weight: 100                            # 服务权重
    ttl: 30                                # 服务生存时间（秒）
    timeout: "10s"                         # 操作超时时间
    config_path: "./conf/polaris.yaml"     # SDK 配置文件路径（可选）
```

### 2. Polaris SDK 配置文件 (polaris.yaml)

这是 Polaris SDK 的标准配置文件，用于配置 SDK 的连接参数：

```yaml
global:
  serverConnector:
    protocol: grpc
    addresses:
      - 127.0.0.1:8091  # Polaris 服务地址
  statReporter:
    enable: true
    chain:
      - prometheus
    plugin:
      prometheus:
        type: push
        address: 127.0.0.1:9091
        interval: 10s

config:
  configConnector:
    addresses:
      - 127.0.0.1:8093  # Polaris 配置中心地址
```

## 配置项说明

### Lynx 配置项

- `namespace`: Polaris 命名空间，用于隔离不同环境或业务的资源
- `token`: 访问 Polaris 服务的认证令牌（可选）
- `weight`: 服务实例的权重，用于负载均衡
- `ttl`: 服务实例的存活时间，用于心跳检测
- `timeout`: 请求 Polaris 服务的超时时间
- `config_path`: Polaris SDK 配置文件的路径（可选）

### Polaris SDK 配置项

#### global 全局配置
- `serverConnector`: 服务连接器配置
  - `protocol`: 连接协议（grpc/http）
  - `addresses`: Polaris 服务地址列表
- `statReporter`: 统计报告器配置
  - `enable`: 是否启用统计报告
  - `chain`: 统计报告链
  - `plugin`: 统计报告插件配置

#### config 配置中心
- `configConnector`: 配置中心连接器
  - `addresses`: 配置中心地址列表

## 使用示例

### 1. 基本配置

```yaml
# 应用配置文件 (config.yaml)
lynx:
  polaris:
    namespace: "default"
    config_path: "./conf/polaris.yaml"
```

```yaml
# Polaris SDK 配置文件 (conf/polaris.yaml)
global:
  serverConnector:
    protocol: grpc
    addresses:
      - 127.0.0.1:8091
  statReporter:
    enable: true
    chain:
      - prometheus
    plugin:
      prometheus:
        type: push
        address: 127.0.0.1:9091
        interval: 10s

config:
  configConnector:
    addresses:
      - 127.0.0.1:8093
```

### 2. 生产环境配置

```yaml
# 应用配置文件
lynx:
  polaris:
    namespace: "production"
    token: "your-production-token"
    weight: 100
    ttl: 30
    timeout: "5s"
    config_path: "./conf/polaris-prod.yaml"
```

```yaml
# 生产环境 Polaris 配置
global:
  serverConnector:
    protocol: grpc
    addresses:
      - polaris-server-1:8091
      - polaris-server-2:8091
      - polaris-server-3:8091
  statReporter:
    enable: true
    chain:
      - prometheus
    plugin:
      prometheus:
        type: push
        address: prometheus-server:9091
        interval: 10s

config:
  configConnector:
    addresses:
      - polaris-config-1:8093
      - polaris-config-2:8093
```

## 注意事项

1. **配置文件路径**: 确保 `config_path` 指向的配置文件存在且可读
2. **服务地址**: 根据你的 Polaris 部署情况修改服务地址
3. **命名空间**: 确保使用正确的命名空间
4. **认证令牌**: 在生产环境中建议使用认证令牌
5. **网络连接**: 确保应用能够访问 Polaris 服务

## 参考文档

- [腾讯北极星官方文档](https://polarismesh.cn/docs)
- [Polaris SDK 配置说明](https://polarismesh.cn/docs/使用指南/服务发现/服务发现SDK/Go-SDK/)
- [Polaris 部署指南](https://polarismesh.cn/docs/使用指南/服务发现/服务发现SDK/Go-SDK/)

## 故障排除

### 常见问题

1. **配置文件未找到**
   - 检查 `config_path` 路径是否正确
   - 确保文件存在且有读取权限

2. **连接失败**
   - 检查 Polaris 服务地址是否正确
   - 确认网络连接是否正常
   - 验证认证令牌是否有效

3. **配置解析错误**
   - 检查 YAML 格式是否正确
   - 确认配置项名称是否正确

### 调试方法

1. 查看应用日志，了解连接状态
2. 使用 Polaris 控制台检查服务注册状态
3. 验证配置文件格式和内容
