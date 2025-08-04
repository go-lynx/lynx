# Kafka 插件重构说明

## 重构目标

本次重构旨在优化 Kafka 插件的代码结构，提高可维护性和可扩展性。

## 文件结构变更

### 重构前
```
kafka/
├── client.go          # 主客户端结构
├── producer.go        # 生产者相关
├── consumer.go        # 消费者相关
├── config.go          # 配置验证
├── errors.go          # 错误定义
├── monitoring.go      # 监控指标和健康检查
├── utils.go           # 工具函数（包含多种功能）
├── constants.go       # 常量定义
└── plug.go           # 插件入口
```

### 重构后
```
kafka/
├── client.go          # 主客户端结构
├── producer.go        # 生产者相关
├── consumer.go        # 消费者相关
├── config.go          # 配置验证
├── errors.go          # 错误定义
├── metrics.go         # 监控指标（从 monitoring.go 分离）
├── health.go          # 健康检查和连接管理（从 monitoring.go 分离）
├── batch.go           # 批量处理（从 utils.go 分离）
├── retry.go           # 重试逻辑（从 utils.go 分离）
├── pool.go            # 协程池（从 utils.go 分离）
├── interfaces.go      # 接口定义（新增）
├── utils.go           # 通用工具函数
├── constants.go       # 常量定义
└── plug.go           # 插件入口
```

## 主要改进

### 1. 文件职责分离

- **metrics.go**: 专注于监控指标收集和管理
- **health.go**: 专注于健康检查和连接管理
- **batch.go**: 专注于批量处理逻辑
- **retry.go**: 专注于重试机制
- **pool.go**: 专注于协程池管理
- **interfaces.go**: 定义清晰的接口契约

### 2. 接口设计

新增了完整的接口定义：

```go
// KafkaProducer 生产者接口
type KafkaProducer interface {
    Produce(ctx context.Context, topic string, key, value []byte) error
    ProduceBatch(ctx context.Context, topic string, records []*kgo.Record) error
    GetProducer() *kgo.Client
    IsProducerReady() bool
}

// KafkaConsumer 消费者接口
type KafkaConsumer interface {
    Subscribe(ctx context.Context, topics []string, handler MessageHandler) error
    GetConsumer() *kgo.Client
    IsConsumerReady() bool
}

// KafkaClientInterface 完整客户端接口
type KafkaClientInterface interface {
    KafkaProducer
    KafkaConsumer
    InitializeResources(rt plugins.Runtime) error
    StartupTasks() error
    ShutdownTasks() error
    GetMetrics() *Metrics
}
```

### 3. 错误处理增强

- 添加了参数验证（如 `validateTopic`）
- 增强了错误包装和上下文信息
- 提供了更详细的错误类型定义

### 4. 配置优化

- 添加了默认配置函数
- 提供了更灵活的配置选项
- 增强了配置验证逻辑

### 5. 监控指标改进

- 分离了指标收集和健康检查
- 提供了更丰富的指标类型
- 支持指标重置和统计

## 使用示例

### 基本使用

```go
// 创建客户端
client := NewKafkaClient()

// 初始化
err := client.InitializeResources(runtime)
if err != nil {
    log.Fatal(err)
}

// 启动
err = client.StartupTasks()
if err != nil {
    log.Fatal(err)
}

// 发送消息
err = client.Produce(ctx, "test-topic", []byte("key"), []byte("value"))
if err != nil {
    log.Error(err)
}

// 订阅消息
err = client.Subscribe(ctx, []string{"test-topic"}, func(ctx context.Context, topic string, partition int32, offset int64, key, value []byte) error {
    log.Infof("Received message: %s", string(value))
    return nil
})
```

### 批量处理

```go
// 创建批量处理器
batchProcessor := NewBatchProcessor(1000, 100*time.Millisecond, func(ctx context.Context, records []*kgo.Record) error {
    // 处理批量记录
    return nil
})

// 添加记录
record := &kgo.Record{
    Topic: "test-topic",
    Key:   []byte("key"),
    Value: []byte("value"),
}
batchProcessor.AddRecord(ctx, record)
```

### 重试机制

```go
// 创建重试处理器
retryHandler := NewRetryHandler(RetryConfig{
    MaxRetries:  3,
    BackoffTime: time.Second,
    MaxBackoff:  30 * time.Second,
})

// 执行带重试的操作
err := retryHandler.DoWithRetry(ctx, func() error {
    // 执行可能失败的操作
    return nil
})
```

## 向后兼容性

重构后的代码保持了向后兼容性：

1. 所有公共 API 保持不变
2. 配置格式保持不变
3. 插件注册方式保持不变

## 性能优化

1. **批量处理**: 支持消息批量发送，提高吞吐量
2. **协程池**: 限制并发处理数量，避免资源耗尽
3. **重试机制**: 智能重试，避免无效重试
4. **连接管理**: 自动重连和健康检查

## 监控和可观测性

1. **详细指标**: 生产/消费消息数、字节数、错误数、延迟等
2. **健康检查**: 定期检查连接状态
3. **错误统计**: 分类统计各种错误类型
4. **性能监控**: 延迟、吞吐量等性能指标

## 后续计划

1. **完善 SASL 认证**: 实现完整的 SASL 认证机制
2. **添加更多压缩算法**: 支持更多压缩类型
3. **增强错误处理**: 添加更多错误类型和处理策略
4. **性能优化**: 进一步优化批量处理和并发性能
5. **测试覆盖**: 添加完整的单元测试和集成测试 