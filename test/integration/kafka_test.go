package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKafkaConnection 测试Kafka连接
func TestKafkaConnection(t *testing.T) {
	// Kafka配置
	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Consumer.Return.Errors = true

	brokers := []string{"localhost:9092"}

	// 创建生产者
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		t.Skip("Kafka is not available:", err)
		return
	}
	defer producer.Close()

	// 创建消费者
	consumer, err := sarama.NewConsumer(brokers, config)
	require.NoError(t, err)
	defer consumer.Close()

	// 测试主题
	topic := "test-topic"

	// 测试生产和消费
	t.Run("ProduceAndConsume", func(t *testing.T) {
		// 发送消息
		message := &sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder("test-key"),
			Value: sarama.StringEncoder("test-value"),
		}

		partition, offset, err := producer.SendMessage(message)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, partition, int32(0))
		assert.GreaterOrEqual(t, offset, int64(0))

		t.Logf("Message sent to partition %d at offset %d", partition, offset)

		// 消费消息
		partitionConsumer, err := consumer.ConsumePartition(topic, partition, offset)
		require.NoError(t, err)
		defer partitionConsumer.Close()

		select {
		case msg := <-partitionConsumer.Messages():
			assert.Equal(t, "test-key", string(msg.Key))
			assert.Equal(t, "test-value", string(msg.Value))
			assert.Equal(t, topic, msg.Topic)
			assert.Equal(t, partition, msg.Partition)
			assert.Equal(t, offset, msg.Offset)
		case err := <-partitionConsumer.Errors():
			t.Fatalf("Error consuming message: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for message")
		}
	})

	// 测试批量生产
	t.Run("BatchProduce", func(t *testing.T) {
		messages := make([]*sarama.ProducerMessage, 10)
		for i := 0; i < 10; i++ {
			messages[i] = &sarama.ProducerMessage{
				Topic: topic,
				Key:   sarama.StringEncoder(fmt.Sprintf("batch-key-%d", i)),
				Value: sarama.StringEncoder(fmt.Sprintf("batch-value-%d", i)),
			}
		}

		// 发送批量消息
		errors := producer.SendMessages(messages)
		assert.NoError(t, errors)

		// 验证所有消息都有有效的分区和偏移量
		for _, msg := range messages {
			assert.GreaterOrEqual(t, msg.Partition, int32(0))
			assert.GreaterOrEqual(t, msg.Offset, int64(0))
		}
	})

	// 测试消费者组
	t.Run("ConsumerGroup", func(t *testing.T) {
		// 创建消费者组
		group, err := sarama.NewConsumerGroup(brokers, "test-group", config)
		require.NoError(t, err)
		defer group.Close()

		// 发送测试消息
		testMsg := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.StringEncoder("group-test-value"),
		}
		_, _, err = producer.SendMessage(testMsg)
		require.NoError(t, err)

		// 消费消息
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		handler := &testConsumerGroupHandler{
			ready: make(chan bool),
			done:  make(chan bool),
		}

		go func() {
			for {
				if err := group.Consume(ctx, []string{topic}, handler); err != nil {
					return
				}
				if ctx.Err() != nil {
					return
				}
			}
		}()

		// 等待消费者准备就绪
		select {
		case <-handler.ready:
			t.Log("Consumer group is ready")
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for consumer group to be ready")
		}
	})
}

// TestKafkaPerformance 测试Kafka性能
func TestKafkaPerformance(t *testing.T) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForLocal
	config.Producer.Compression = sarama.CompressionSnappy

	brokers := []string{"localhost:9092"}

	// 创建异步生产者
	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		t.Skip("Kafka is not available:", err)
		return
	}
	defer producer.Close()

	topic := "perf-test-topic"

	// 性能测试：异步生产
	t.Run("AsyncProduce", func(t *testing.T) {
		messageCount := 10000
		start := time.Now()

		var wg sync.WaitGroup
		wg.Add(1)

		// 处理成功和错误
		go func() {
			defer wg.Done()
			successCount := 0
			errorCount := 0

			for successCount+errorCount < messageCount {
				select {
				case <-producer.Successes():
					successCount++
				case err := <-producer.Errors():
					errorCount++
					t.Logf("Error producing message: %v", err)
				case <-time.After(10 * time.Second):
					t.Logf("Timeout: received %d successes, %d errors", successCount, errorCount)
					return
				}
			}

			elapsed := time.Since(start)
			throughput := float64(successCount) / elapsed.Seconds()

			t.Logf("Async produce performance: %d messages in %v (%.2f msg/sec)",
				successCount, elapsed, throughput)

			// 验证性能阈值
			assert.Greater(t, throughput, 1000.0, "Should achieve at least 1000 msg/sec")
			assert.Equal(t, 0, errorCount, "Should have no errors")
		}()

		// 发送消息
		for i := 0; i < messageCount; i++ {
			message := &sarama.ProducerMessage{
				Topic: topic,
				Key:   sarama.StringEncoder(fmt.Sprintf("key-%d", i)),
				Value: sarama.StringEncoder(fmt.Sprintf("value-%d", i)),
			}

			select {
			case producer.Input() <- message:
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout sending message")
			}
		}

		wg.Wait()
	})

	// 性能测试：批量消费
	t.Run("BatchConsume", func(t *testing.T) {
		consumer, err := sarama.NewConsumer(brokers, config)
		require.NoError(t, err)
		defer consumer.Close()

		// 获取主题的所有分区
		partitions, err := consumer.Partitions(topic)
		require.NoError(t, err)
		require.NotEmpty(t, partitions)

		// 消费第一个分区
		partitionConsumer, err := consumer.ConsumePartition(topic, partitions[0], sarama.OffsetOldest)
		require.NoError(t, err)
		defer partitionConsumer.Close()

		start := time.Now()
		consumed := 0
		timeout := time.After(5 * time.Second)

		for consumed < 1000 {
			select {
			case <-partitionConsumer.Messages():
				consumed++
			case err := <-partitionConsumer.Errors():
				t.Logf("Error consuming: %v", err)
			case <-timeout:
				goto done
			}
		}

	done:
		elapsed := time.Since(start)
		throughput := float64(consumed) / elapsed.Seconds()

		t.Logf("Batch consume performance: %d messages in %v (%.2f msg/sec)",
			consumed, elapsed, throughput)

		// 验证性能阈值
		assert.Greater(t, throughput, 1000.0, "Should achieve at least 1000 msg/sec consumption")
	})
}

// TestKafkaReliability 测试Kafka可靠性
func TestKafkaReliability(t *testing.T) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll // 等待所有副本确认
	config.Producer.Retry.Max = 10
	config.Producer.Idempotent = true // 幂等性

	brokers := []string{"localhost:9092"}

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		t.Skip("Kafka is not available:", err)
		return
	}
	defer producer.Close()

	topic := "reliability-test-topic"

	// 测试消息顺序性
	t.Run("MessageOrdering", func(t *testing.T) {
		// 使用相同的key确保消息发送到同一分区
		key := "order-key"
		messages := make([]*sarama.ProducerMessage, 10)

		for i := 0; i < 10; i++ {
			messages[i] = &sarama.ProducerMessage{
				Topic: topic,
				Key:   sarama.StringEncoder(key),
				Value: sarama.StringEncoder(fmt.Sprintf("message-%d", i)),
			}
		}

		// 发送消息并记录分区和偏移量
		var partition int32
		offsets := make([]int64, 10)

		for i, msg := range messages {
			p, o, err := producer.SendMessage(msg)
			require.NoError(t, err)

			if i == 0 {
				partition = p
			} else {
				// 验证所有消息都发送到同一分区
				assert.Equal(t, partition, p, "Messages with same key should go to same partition")
			}

			offsets[i] = o
		}

		// 验证偏移量是连续递增的
		for i := 1; i < len(offsets); i++ {
			assert.Equal(t, offsets[i-1]+1, offsets[i], "Offsets should be sequential")
		}

		t.Logf("All messages sent to partition %d with sequential offsets", partition)
	})

	// 测试幂等性
	t.Run("Idempotency", func(t *testing.T) {
		message := &sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder("idempotent-key"),
			Value: sarama.StringEncoder("idempotent-value"),
		}

		// 发送相同消息多次
		partition1, offset1, err1 := producer.SendMessage(message)
		require.NoError(t, err1)

		// 由于幂等性，重复发送会得到不同的偏移量
		partition2, offset2, err2 := producer.SendMessage(message)
		require.NoError(t, err2)

		assert.Equal(t, partition1, partition2, "Should send to same partition")
		assert.NotEqual(t, offset1, offset2, "Should have different offsets")

		t.Logf("Idempotent messages sent: offset1=%d, offset2=%d", offset1, offset2)
	})
}

// testConsumerGroupHandler 实现ConsumerGroupHandler接口
type testConsumerGroupHandler struct {
	ready chan bool
	done  chan bool
}

func (h *testConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

func (h *testConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *testConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		session.MarkMessage(message, "")
		select {
		case h.done <- true:
		default:
		}
	}
	return nil
}
