package snowflake

import (
	"time"

	pb "github.com/go-lynx/lynx/plugins/snowflake/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// 测试配置常量
const (
	TestRedisAddr     = "localhost:6379"
	TestRedisDB       = 15 // 使用专门的测试数据库
	TestKeyPrefix     = "test:snowflake"
	TestMaxWorkers    = 1024
	TestHeartbeatTTL  = 30 * time.Second
	TestHeartbeatInterval = 10 * time.Second
)

// GetTestConfig 获取测试配置
func GetTestConfig(workerID, datacenterID int32) *pb.Snowflake {
	return &pb.Snowflake{
		WorkerId:                       workerID,
		DatacenterId:                   datacenterID,
		AutoRegisterWorkerId:           false,
		RedisKeyPrefix:                 TestKeyPrefix,
		RedisPluginName:                "redis",
		WorkerIdTtl:                    durationpb.New(TestHeartbeatTTL),
		HeartbeatInterval:              durationpb.New(TestHeartbeatInterval),
		EnableClockDriftProtection:     true,
	}
}

// GetTestConfigWithAutoWorkerID 获取自动分配WorkerID的测试配置
func GetTestConfigWithAutoWorkerID(datacenterID int32) *pb.Snowflake {
	config := GetTestConfig(0, datacenterID)
	config.AutoRegisterWorkerId = true
	return config
}

// GetTestConfigWithoutRedis 获取不使用Redis的测试配置
func GetTestConfigWithoutRedis(workerID, datacenterID int32) *pb.Snowflake {
	return &pb.Snowflake{
		WorkerId:                       workerID,
		DatacenterId:                   datacenterID,
		AutoRegisterWorkerId:           false,
		RedisPluginName:                "",
		EnableClockDriftProtection:     true,
	}
}

// GetTestGeneratorConfig 获取生成器测试配置
func GetTestGeneratorConfig(workerID, datacenterID int64) *GeneratorConfig {
	return &GeneratorConfig{
		CustomEpoch:                DefaultEpoch,
		DatacenterIDBits:          5,
		WorkerIDBits:              5,
		SequenceBits:              12,
		EnableClockDriftProtection: true,
		ClockDriftAction:          ClockDriftActionWait,
	}
}

// GetTestWorkerManagerConfig 获取WorkerID管理器测试配置
func GetTestWorkerManagerConfig() *WorkerManagerConfig {
	return &WorkerManagerConfig{
		KeyPrefix:         TestKeyPrefix,
		TTL:               TestHeartbeatTTL,
		HeartbeatInterval: TestHeartbeatInterval,
	}
}