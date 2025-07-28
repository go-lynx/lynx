# Redis 分布式锁 (重构版)

基于 Redis 的分布式锁实现，按领域分离的模块化设计。

## 文件结构

```
redislock_v2/
├── errors.go      # 错误定义
├── types.go       # 类型定义和接口
├── scripts.go     # Lua 脚本
├── utils.go       # 工具函数
├── manager.go     # 锁管理器
├── lock.go        # 锁实例方法
├── api.go         # 公共 API
└── README.md      # 文档
```

## 模块说明

### 1. errors.go - 错误定义
- 集中定义所有锁相关的错误类型
- 便于错误处理和国际化

### 2. types.go - 类型定义
- `LockOptions`: 锁配置选项
- `RetryStrategy`: 重试策略
- `LockCallback`: 监控回调接口
- `RedisLock`: 锁实例结构
- `lockManager`: 锁管理器结构
- 默认配置常量

### 3. scripts.go - Lua 脚本
- `lockScript`: 获取锁的脚本
- `unlockScript`: 释放锁的脚本
- `renewScript`: 续期锁的脚本
- 支持可重入锁扩展

### 4. utils.go - 工具函数
- 锁值生成逻辑
- 进程标识初始化
- 主机名和IP获取

### 5. manager.go - 锁管理器
- 全局锁管理器实例
- 续期服务管理
- 工作池模式
- 统计信息收集
- 优雅关闭

### 6. lock.go - 锁实例方法
- 锁状态查询方法
- 手动续期和释放
- 锁状态检查

### 7. api.go - 公共 API
- 主要的锁操作接口
- 向后兼容的 API
- 错误处理和回调

## 设计优势

### 1. **模块化设计**
- 每个文件职责单一，便于维护
- 代码复用性更好
- 测试更容易

### 2. **清晰的依赖关系**
```
api.go → manager.go → lock.go
    ↓         ↓         ↓
types.go → scripts.go → utils.go
    ↓
errors.go
```

### 3. **易于扩展**
- 新增功能只需修改对应模块
- 不影响其他模块
- 便于添加新特性

### 4. **更好的可读性**
- 代码组织更清晰
- 文件大小适中
- 便于团队协作

## 使用方式

使用方式与原版本完全相同，只是内部结构更加模块化：

```go
import "github.com/go-lynx/lynx/plugins/nosql/redis/redislock"

// 基本使用
err := redislock.Lock(context.Background(), "my-lock", 30*time.Second, func() error {
    // 业务逻辑
    return nil
})

// 带配置的使用
options := redislock.LockOptions{
    Expiration:       60 * time.Second,
    RetryStrategy:    redislock.DefaultRetryStrategy,
    RenewalEnabled:   true,
    RenewalThreshold: 0.5,
}

err := redislock.LockWithOptions(context.Background(), "my-lock", options, func() error {
    // 业务逻辑
    return nil
})
```

## 迁移指南

从原版本迁移到重构版本：

1. **导入路径保持不变**
2. **API 接口完全兼容**
3. **配置选项保持一致**
4. **错误处理方式相同**

## 性能优化

重构后的版本保持了所有性能优化：

- ✅ 工作池模式限制并发
- ✅ 智能续期检查
- ✅ 指数退避重试
- ✅ 高频检查响应
- ✅ 原子操作统计

## 维护建议

1. **错误处理**: 在 `errors.go` 中统一管理错误
2. **类型扩展**: 在 `types.go` 中添加新的类型定义
3. **脚本优化**: 在 `scripts.go` 中优化 Lua 脚本
4. **功能增强**: 在对应模块中添加新功能
5. **测试覆盖**: 为每个模块编写单元测试 