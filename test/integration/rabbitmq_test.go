package integration

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRabbitMQConnection 测试RabbitMQ连接
func TestRabbitMQConnection(t *testing.T) {
	// 连接RabbitMQ
	conn, err := amqp.Dial("amqp://lynx:lynx123456@localhost:5672/")
	if err != nil {
		t.Skip("RabbitMQ is not available:", err)
		return
	}
	defer conn.Close()

	// 创建通道
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// 测试基本发布和消费
	t.Run("BasicPublishConsume", func(t *testing.T) {
		queueName := "test-queue"

		// 声明队列
		q, err := ch.QueueDeclare(
			queueName, // 名称
			false,     // 持久化
			true,      // 自动删除
			false,     // 独占
			false,     // 不等待
			nil,       // 参数
		)
		require.NoError(t, err)

		// 发布消息
		body := "Hello RabbitMQ!"
		err = ch.Publish(
			"",     // exchange
			q.Name, // routing key
			false,  // mandatory
			false,  // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(body),
			},
		)
		assert.NoError(t, err)

		// 消费消息
		msgs, err := ch.Consume(
			q.Name, // queue
			"",     // consumer
			true,   // auto-ack
			false,  // exclusive
			false,  // no-local
			false,  // no-wait
			nil,    // args
		)
		require.NoError(t, err)

		// 验证消息
		select {
		case msg := <-msgs:
			assert.Equal(t, body, string(msg.Body))
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for message")
		}
	})

	// 测试交换机和路由
	t.Run("ExchangeRouting", func(t *testing.T) {
		exchangeName := "test-exchange"

		// 声明交换机
		err := ch.ExchangeDeclare(
			exchangeName, // name
			"direct",     // type
			false,        // durable
			true,         // auto-deleted
			false,        // internal
			false,        // no-wait
			nil,          // arguments
		)
		require.NoError(t, err)

		// 创建两个队列
		q1, err := ch.QueueDeclare("queue1", false, true, false, false, nil)
		require.NoError(t, err)

		q2, err := ch.QueueDeclare("queue2", false, true, false, false, nil)
		require.NoError(t, err)

		// 绑定队列到交换机
		err = ch.QueueBind(q1.Name, "route1", exchangeName, false, nil)
		require.NoError(t, err)

		err = ch.QueueBind(q2.Name, "route2", exchangeName, false, nil)
		require.NoError(t, err)

		// 发布消息到不同路由
		err = ch.Publish(
			exchangeName, // exchange
			"route1",     // routing key
			false,        // mandatory
			false,        // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte("Message for queue1"),
			},
		)
		assert.NoError(t, err)

		err = ch.Publish(
			exchangeName, // exchange
			"route2",     // routing key
			false,        // mandatory
			false,        // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte("Message for queue2"),
			},
		)
		assert.NoError(t, err)

		// 验证消息路由正确
		msgs1, err := ch.Consume(q1.Name, "", true, false, false, false, nil)
		require.NoError(t, err)

		msgs2, err := ch.Consume(q2.Name, "", true, false, false, false, nil)
		require.NoError(t, err)

		select {
		case msg := <-msgs1:
			assert.Equal(t, "Message for queue1", string(msg.Body))
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for message in queue1")
		}

		select {
		case msg := <-msgs2:
			assert.Equal(t, "Message for queue2", string(msg.Body))
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for message in queue2")
		}
	})

	// 测试消息确认
	t.Run("MessageAcknowledgment", func(t *testing.T) {
		queueName := "ack-queue"

		q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
		require.NoError(t, err)

		// 发布消息
		err = ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType:  "text/plain",
			Body:         []byte("ack test message"),
			DeliveryMode: amqp.Persistent, // 持久化消息
		})
		require.NoError(t, err)

		// 消费消息（手动确认）
		msgs, err := ch.Consume(
			q.Name,
			"",
			false, // auto-ack = false
			false,
			false,
			false,
			nil,
		)
		require.NoError(t, err)

		select {
		case msg := <-msgs:
			// 手动确认消息
			err := msg.Ack(false)
			assert.NoError(t, err)
			assert.Equal(t, "ack test message", string(msg.Body))
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for message")
		}

		// 清理队列
		ch.QueueDelete(queueName, false, false, false)
	})
}

// TestRabbitMQPerformance 测试RabbitMQ性能
func TestRabbitMQPerformance(t *testing.T) {
	conn, err := amqp.Dial("amqp://lynx:lynx123456@localhost:5672/")
	if err != nil {
		t.Skip("RabbitMQ is not available:", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// 设置QoS
	err = ch.Qos(100, 0, false)
	require.NoError(t, err)

	// 批量发布性能测试
	t.Run("BatchPublish", func(t *testing.T) {
		queueName := "perf-queue"

		// 声明持久化队列
		q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
		require.NoError(t, err)
		defer ch.QueueDelete(queueName, false, false, false)

		messageCount := 10000
		start := time.Now()

		// 批量发布
		for i := 0; i < messageCount; i++ {
			err := ch.Publish(
				"",
				q.Name,
				false,
				false,
				amqp.Publishing{
					ContentType:  "text/plain",
					Body:         []byte(fmt.Sprintf("message-%d", i)),
					DeliveryMode: amqp.Transient, // 非持久化以提高性能
				},
			)
			if err != nil {
				t.Logf("Error publishing message %d: %v", i, err)
			}
		}

		elapsed := time.Since(start)
		throughput := float64(messageCount) / elapsed.Seconds()

		t.Logf("Batch publish performance: %d messages in %v (%.2f msg/sec)",
			messageCount, elapsed, throughput)

		// 验证性能阈值
		assert.Greater(t, throughput, 1000.0, "Should achieve at least 1000 msg/sec")
	})

	// 并发消费性能测试
	t.Run("ConcurrentConsume", func(t *testing.T) {
		queueName := "concurrent-queue"

		q, err := ch.QueueDeclare(queueName, false, true, false, false, nil)
		require.NoError(t, err)

		// 预先发布消息
		messageCount := 1000
		for i := 0; i < messageCount; i++ {
			ch.Publish("", q.Name, false, false, amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(fmt.Sprintf("concurrent-%d", i)),
			})
		}

		// 创建多个消费者
		consumerCount := 5
		var wg sync.WaitGroup
		var consumed int32

		start := time.Now()

		for c := 0; c < consumerCount; c++ {
			wg.Add(1)
			go func(consumerID int) {
				defer wg.Done()

				// 每个消费者使用独立的通道
				ch, err := conn.Channel()
				if err != nil {
					t.Logf("Consumer %d: failed to create channel: %v", consumerID, err)
					return
				}
				defer ch.Close()

				msgs, err := ch.Consume(
					q.Name,
					fmt.Sprintf("consumer-%d", consumerID),
					true,
					false,
					false,
					false,
					nil,
				)
				if err != nil {
					t.Logf("Consumer %d: failed to consume: %v", consumerID, err)
					return
				}

				timeout := time.After(2 * time.Second)
				for {
					select {
					case msg, ok := <-msgs:
						if !ok {
							return
						}
						atomic.AddInt32(&consumed, 1)
						_ = msg
					case <-timeout:
						return
					}
				}
			}(c)
		}

		wg.Wait()

		elapsed := time.Since(start)
		throughput := float64(consumed) / elapsed.Seconds()

		t.Logf("Concurrent consume performance: %d messages consumed in %v (%.2f msg/sec)",
			consumed, elapsed, throughput)

		// 验证性能阈值
		assert.Greater(t, throughput, 500.0, "Should achieve at least 500 msg/sec with concurrent consumers")
	})
}

// TestRabbitMQReliability 测试RabbitMQ可靠性
func TestRabbitMQReliability(t *testing.T) {
	conn, err := amqp.Dial("amqp://lynx:lynx123456@localhost:5672/")
	if err != nil {
		t.Skip("RabbitMQ is not available:", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// 测试消息持久化
	t.Run("MessagePersistence", func(t *testing.T) {
		queueName := "persistent-queue"

		// 声明持久化队列
		q, err := ch.QueueDeclare(
			queueName,
			true,  // durable
			false, // auto-delete
			false,
			false,
			nil,
		)
		require.NoError(t, err)
		defer ch.QueueDelete(queueName, false, false, false)

		// 发布持久化消息
		err = ch.Publish(
			"",
			q.Name,
			false,
			false,
			amqp.Publishing{
				ContentType:  "text/plain",
				Body:         []byte("persistent message"),
				DeliveryMode: amqp.Persistent,
			},
		)
		assert.NoError(t, err)

		// 获取队列状态
		q, err = ch.QueueInspect(queueName)
		assert.NoError(t, err)
		assert.Equal(t, 1, q.Messages, "Should have 1 message in queue")
	})

	// 测试发布确认
	t.Run("PublisherConfirms", func(t *testing.T) {
		// 启用发布确认模式
		err := ch.Confirm(false)
		require.NoError(t, err)

		queueName := "confirm-queue"
		q, err := ch.QueueDeclare(queueName, false, true, false, false, nil)
		require.NoError(t, err)

		// 发布消息并等待确认
		confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

		err = ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("confirm test"),
		})
		require.NoError(t, err)

		// 等待确认
		select {
		case confirm := <-confirms:
			assert.True(t, confirm.Ack, "Message should be acknowledged")
			t.Logf("Message confirmed with tag: %d", confirm.DeliveryTag)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for publish confirmation")
		}
	})

	// 测试死信队列
	t.Run("DeadLetterQueue", func(t *testing.T) {
		// 创建死信交换机和队列
		dlxName := "dlx-exchange"
		dlqName := "dead-letter-queue"

		err := ch.ExchangeDeclare(dlxName, "direct", true, false, false, false, nil)
		require.NoError(t, err)

		dlq, err := ch.QueueDeclare(dlqName, true, false, false, false, nil)
		require.NoError(t, err)
		defer ch.QueueDelete(dlqName, false, false, false)

		err = ch.QueueBind(dlq.Name, "failed", dlxName, false, nil)
		require.NoError(t, err)

		// 创建带死信配置的主队列
		mainQueue := "main-queue-with-dlx"
		q, err := ch.QueueDeclare(
			mainQueue,
			true,
			false,
			false,
			false,
			amqp.Table{
				"x-dead-letter-exchange":    dlxName,
				"x-dead-letter-routing-key": "failed",
				"x-message-ttl":             1000, // 1秒过期
			},
		)
		require.NoError(t, err)
		defer ch.QueueDelete(mainQueue, false, false, false)

		// 发送消息到主队列
		err = ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("message to expire"),
		})
		require.NoError(t, err)

		// 等待消息过期并进入死信队列
		time.Sleep(2 * time.Second)

		// 检查死信队列
		dlqStatus, err := ch.QueueInspect(dlqName)
		assert.NoError(t, err)
		assert.Equal(t, 1, dlqStatus.Messages, "Should have 1 message in dead letter queue")
	})
}
