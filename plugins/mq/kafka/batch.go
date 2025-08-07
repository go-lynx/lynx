package kafka

import (
	"context"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/twmb/franz-go/pkg/kgo"
)

// BatchProcessor 批量处理器
type BatchProcessor struct {
	maxBatchSize int
	maxWaitTime  time.Duration
	handler      func(context.Context, []*kgo.Record) error
	records      []*kgo.Record
	mu           sync.Mutex
	timer        *time.Timer
	done         chan struct{}
}

// NewBatchProcessor 创建新的批量处理器
func NewBatchProcessor(maxBatchSize int, maxWaitTime time.Duration, handler func(context.Context, []*kgo.Record) error) *BatchProcessor {
	bp := &BatchProcessor{
		maxBatchSize: maxBatchSize,
		maxWaitTime:  maxWaitTime,
		handler:      handler,
		records:      make([]*kgo.Record, 0, maxBatchSize),
		done:         make(chan struct{}),
	}
	return bp
}

// AddRecord 添加记录到批量处理器
func (bp *BatchProcessor) AddRecord(ctx context.Context, record *kgo.Record) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.records = append(bp.records, record)

	// 如果达到最大批量大小，立即处理
	if len(bp.records) >= bp.maxBatchSize {
		return bp.processBatch(ctx)
	}

	// 设置定时器，在最大等待时间后处理
	if bp.timer == nil {
		bp.timer = time.AfterFunc(bp.maxWaitTime, func() {
			bp.mu.Lock()
			defer bp.mu.Unlock()
			if len(bp.records) > 0 {
				err := bp.processBatch(ctx)
				if err != nil {
					return
				}
			}
		})
	}

	return nil
}

// processBatch 处理批量记录
func (bp *BatchProcessor) processBatch(ctx context.Context) error {
	if len(bp.records) == 0 {
		return nil
	}

	// 停止定时器
	if bp.timer != nil {
		bp.timer.Stop()
		bp.timer = nil
	}

	// 复制记录并清空原切片
	records := make([]*kgo.Record, len(bp.records))
	copy(records, bp.records)
	bp.records = bp.records[:0]

	// 异步处理批量记录
	go func() {
		if err := bp.handler(ctx, records); err != nil {
			log.ErrorfCtx(ctx, "Batch processing failed: %v", err)
		}
	}()

	return nil
}

// Flush 强制处理所有待处理的记录
func (bp *BatchProcessor) Flush(ctx context.Context) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.processBatch(ctx)
}

// Close 关闭批量处理器
func (bp *BatchProcessor) Close() {
	close(bp.done)
}

// BatchConfig 批量处理配置
type BatchConfig struct {
	MaxBatchSize int           // 最大批量大小
	MaxWaitTime  time.Duration // 最大等待时间
	Compression  string        // 压缩类型
	RetryCount   int           // 重试次数
}

// DefaultBatchConfig 默认批量处理配置
func DefaultBatchConfig() *BatchConfig {
	return &BatchConfig{
		MaxBatchSize: 1000,
		MaxWaitTime:  100 * time.Millisecond,
		Compression:  "none",
		RetryCount:   3,
	}
}
