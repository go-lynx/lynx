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

// TestRabbitMQConnection tests RabbitMQ connection
func TestRabbitMQConnection(t *testing.T) {
	// Connect to RabbitMQ
	conn, err := amqp.Dial("amqp://lynx:lynx123456@localhost:5672/")
	if err != nil {
		t.Skip("RabbitMQ is not available:", err)
		return
	}
	defer conn.Close()

	// Create channel
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// Test basic publish and consume
	t.Run("BasicPublishConsume", func(t *testing.T) {
		queueName := "test-queue"

		// Declare queue
		q, err := ch.QueueDeclare(
			queueName, // name
			false,     // durable
			true,      // auto-delete
			false,     // exclusive
			false,     // no-wait
			nil,       // args
		)
		require.NoError(t, err)

		// Publish message
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

		// Consume message
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

		// Verify message
		select {
		case msg := <-msgs:
			assert.Equal(t, body, string(msg.Body))
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for message")
		}
	})

	// Test exchange and routing
	t.Run("ExchangeRouting", func(t *testing.T) {
		exchangeName := "test-exchange"

		// Declare exchange
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

		// Create two queues
		q1, err := ch.QueueDeclare("queue1", false, true, false, false, nil)
		require.NoError(t, err)

		q2, err := ch.QueueDeclare("queue2", false, true, false, false, nil)
		require.NoError(t, err)

		// Bind queues to exchange
		err = ch.QueueBind(q1.Name, "route1", exchangeName, false, nil)
		require.NoError(t, err)

		err = ch.QueueBind(q2.Name, "route2", exchangeName, false, nil)
		require.NoError(t, err)

		// Publish messages to different routes
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

		// Verify message routing is correct
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

	// Test message acknowledgment
	t.Run("MessageAcknowledgment", func(t *testing.T) {
		queueName := "ack-queue"

		q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
		require.NoError(t, err)

		// Publish message
		err = ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType:  "text/plain",
			Body:         []byte("ack test message"),
			DeliveryMode: amqp.Persistent, // persistent message
		})
		require.NoError(t, err)

		// Consume message (manual acknowledgment)
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
			// Manually acknowledge message
			err := msg.Ack(false)
			assert.NoError(t, err)
			assert.Equal(t, "ack test message", string(msg.Body))
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for message")
		}

		// Clean up queue
		ch.QueueDelete(queueName, false, false, false)
	})
}

// TestRabbitMQPerformance tests RabbitMQ performance
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

	// Set QoS
	err = ch.Qos(100, 0, false)
	require.NoError(t, err)

	// Batch publish performance test
	t.Run("BatchPublish", func(t *testing.T) {
		queueName := "perf-queue"

		// Declare durable queue
		q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
		require.NoError(t, err)
		defer ch.QueueDelete(queueName, false, false, false)

		messageCount := 10000
		start := time.Now()

		// Batch publish
		for i := 0; i < messageCount; i++ {
			err := ch.Publish(
				"",
				q.Name,
				false,
				false,
				amqp.Publishing{
					ContentType:  "text/plain",
					Body:         []byte(fmt.Sprintf("message-%d", i)),
					DeliveryMode: amqp.Transient, // non-persistent for better performance
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

		// Verify performance threshold
		assert.Greater(t, throughput, 1000.0, "Should achieve at least 1000 msg/sec")
	})

	// Concurrent consume performance test
	t.Run("ConcurrentConsume", func(t *testing.T) {
		queueName := "concurrent-queue"

		q, err := ch.QueueDeclare(queueName, false, true, false, false, nil)
		require.NoError(t, err)

		// Pre-publish messages
		messageCount := 1000
		for i := 0; i < messageCount; i++ {
			ch.Publish("", q.Name, false, false, amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(fmt.Sprintf("concurrent-%d", i)),
			})
		}

		// Create multiple consumers
		consumerCount := 5
		var wg sync.WaitGroup
		var consumed int32

		start := time.Now()

		for c := 0; c < consumerCount; c++ {
			wg.Add(1)
			go func(consumerID int) {
				defer wg.Done()

				// Each consumer uses an independent channel
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

		// Verify performance threshold
		assert.Greater(t, throughput, 500.0, "Should achieve at least 500 msg/sec with concurrent consumers")
	})
}

// TestRabbitMQReliability tests RabbitMQ reliability
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

	// Test message persistence
	t.Run("MessagePersistence", func(t *testing.T) {
		queueName := "persistent-queue"

		// Declare durable queue
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

		// Publish persistent message
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

		// Get queue status
		q, err = ch.QueueInspect(queueName)
		assert.NoError(t, err)
		assert.Equal(t, 1, q.Messages, "Should have 1 message in queue")
	})

	// Test publisher confirms
	t.Run("PublisherConfirms", func(t *testing.T) {
		// Enable publisher confirm mode
		err := ch.Confirm(false)
		require.NoError(t, err)

		queueName := "confirm-queue"
		q, err := ch.QueueDeclare(queueName, false, true, false, false, nil)
		require.NoError(t, err)

		// Publish message and wait for confirmation
		confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

		err = ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("confirm test"),
		})
		require.NoError(t, err)

		// Wait for confirmation
		select {
		case confirm := <-confirms:
			assert.True(t, confirm.Ack, "Message should be acknowledged")
			t.Logf("Message confirmed with tag: %d", confirm.DeliveryTag)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for publish confirmation")
		}
	})

	// Test dead letter queue
	t.Run("DeadLetterQueue", func(t *testing.T) {
		// Create dead letter exchange and queue
		dlxName := "dlx-exchange"
		dlqName := "dead-letter-queue"

		err := ch.ExchangeDeclare(dlxName, "direct", true, false, false, false, nil)
		require.NoError(t, err)

		dlq, err := ch.QueueDeclare(dlqName, true, false, false, false, nil)
		require.NoError(t, err)
		defer ch.QueueDelete(dlqName, false, false, false)

		err = ch.QueueBind(dlq.Name, "failed", dlxName, false, nil)
		require.NoError(t, err)

		// Create main queue with dead letter configuration
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
				"x-message-ttl":             1000, // expire after 1 second
			},
		)
		require.NoError(t, err)
		defer ch.QueueDelete(mainQueue, false, false, false)

		// Send message to main queue
		err = ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("message to expire"),
		})
		require.NoError(t, err)

		// Wait for message to expire and enter dead letter queue
		time.Sleep(2 * time.Second)

		// Check dead letter queue
		dlqStatus, err := ch.QueueInspect(dlqName)
		assert.NoError(t, err)
		assert.Equal(t, 1, dlqStatus.Messages, "Should have 1 message in dead letter queue")
	})
}
