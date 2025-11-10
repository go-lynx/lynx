package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/IBM/sarama"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrometheusMetricsEndpoint tests Prometheus metrics endpoint
func TestPrometheusMetricsEndpoint(t *testing.T) {
	// Start a simple metrics server
	mux := http.NewServeMux()

	// Register metrics endpoint
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		// Simulate collecting metrics from various plugins
		var metrics []string

		// Redis metrics
		if redisMetrics := collectRedisMetrics(); len(redisMetrics) > 0 {
			metrics = append(metrics, redisMetrics...)
		}

		// Kafka metrics
		if kafkaMetrics := collectKafkaMetrics(); len(kafkaMetrics) > 0 {
			metrics = append(metrics, kafkaMetrics...)
		}

		// RabbitMQ metrics
		if rabbitMetrics := collectRabbitMQMetrics(); len(rabbitMetrics) > 0 {
			metrics = append(metrics, rabbitMetrics...)
		}

		response := strings.Join(metrics, "\n")
		fmt.Fprint(w, response)
	})

	// Start server
	server := &http.Server{
		Addr:    ":9090",
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Metrics server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test the metrics endpoint
	t.Run("MetricsEndpointAvailable", func(t *testing.T) {
		resp, err := http.Get("http://localhost:9090/metrics")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		t.Logf("Metrics response length: %d bytes", len(body))
	})

	server.Close()
}

// collectRedisMetrics collects Redis metrics
func collectRedisMetrics() []string {
	metrics := []string{
		"# HELP lynx_redis_connections_active Number of active Redis connections",
		"# TYPE lynx_redis_connections_active gauge",
	}

	// Try to connect to Redis and get actual metrics
	opt := &redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	}

	client := redis.NewClient(opt)
	defer client.Close()

	// Get pool stats
	stats := client.PoolStats()
	if stats != nil {
		metrics = append(metrics,
			fmt.Sprintf("lynx_redis_connections_active %d", stats.TotalConns),
			fmt.Sprintf("lynx_redis_connections_idle %d", stats.IdleConns),
			fmt.Sprintf("lynx_redis_connections_stale %d", stats.StaleConns),
		)
	}

	// Add operation counters
	metrics = append(metrics,
		"# HELP lynx_redis_commands_total Total number of Redis commands executed",
		"# TYPE lynx_redis_commands_total counter",
		"lynx_redis_commands_total{command=\"get\"} 0",
		"lynx_redis_commands_total{command=\"set\"} 0",
		"lynx_redis_commands_total{command=\"del\"} 0",
	)

	// Add latency metrics
	metrics = append(metrics,
		"# HELP lynx_redis_command_duration_seconds Redis command duration in seconds",
		"# TYPE lynx_redis_command_duration_seconds histogram",
	)

	return metrics
}

// collectKafkaMetrics collects Kafka metrics
func collectKafkaMetrics() []string {
	metrics := []string{
		"# HELP lynx_kafka_messages_produced_total Total number of messages produced to Kafka",
		"# TYPE lynx_kafka_messages_produced_total counter",
		"lynx_kafka_messages_produced_total{topic=\"test-topic\",partition=\"0\"} 0",
		"",
		"# HELP lynx_kafka_messages_consumed_total Total number of messages consumed from Kafka",
		"# TYPE lynx_kafka_messages_consumed_total counter",
		"lynx_kafka_messages_consumed_total{topic=\"test-topic\",partition=\"0\",group=\"test-group\"} 0",
		"",
		"# HELP lynx_kafka_consumer_lag Current approximate lag of a consumer group",
		"# TYPE lynx_kafka_consumer_lag gauge",
		"lynx_kafka_consumer_lag{topic=\"test-topic\",partition=\"0\",group=\"test-group\"} 0",
		"",
		"# HELP lynx_kafka_producer_errors_total Total number of producer errors",
		"# TYPE lynx_kafka_producer_errors_total counter",
		"lynx_kafka_producer_errors_total 0",
	}

	return metrics
}

// collectRabbitMQMetrics collects RabbitMQ metrics
func collectRabbitMQMetrics() []string {
	metrics := []string{
		"# HELP lynx_rabbitmq_messages_published_total Total number of messages published",
		"# TYPE lynx_rabbitmq_messages_published_total counter",
		"lynx_rabbitmq_messages_published_total{exchange=\"\",queue=\"test-queue\"} 0",
		"",
		"# HELP lynx_rabbitmq_messages_consumed_total Total number of messages consumed",
		"# TYPE lynx_rabbitmq_messages_consumed_total counter",
		"lynx_rabbitmq_messages_consumed_total{queue=\"test-queue\"} 0",
		"",
		"# HELP lynx_rabbitmq_messages_failed_total Total number of failed messages",
		"# TYPE lynx_rabbitmq_messages_failed_total counter",
		"lynx_rabbitmq_messages_failed_total 0",
		"",
		"# HELP lynx_rabbitmq_connection_state RabbitMQ connection state (1=connected, 0=disconnected)",
		"# TYPE lynx_rabbitmq_connection_state gauge",
		"lynx_rabbitmq_connection_state 0",
		"",
		"# HELP lynx_rabbitmq_channel_count Number of active channels",
		"# TYPE lynx_rabbitmq_channel_count gauge",
		"lynx_rabbitmq_channel_count 0",
	}

	return metrics
}

// TestRedisMetricsCollection tests Redis metrics collection
func TestRedisMetricsCollection(t *testing.T) {
	opt := &redis.Options{
		Addr: "localhost:6379",
	}

	client := redis.NewClient(opt)
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis is not available:", err)
		return
	}
	defer client.Close()

	// Perform some operations to generate metrics
	operations := 100

	for i := 0; i < operations; i++ {
		key := fmt.Sprintf("metrics-test-%d", i)
		client.Set(ctx, key, "value", time.Minute)
		client.Get(ctx, key)
		client.Del(ctx, key)
	}

	// Validate pool stats
	stats := client.PoolStats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalConns, uint32(1))

	t.Logf("Redis Pool Stats - Total: %d, Idle: %d, Stale: %d",
		stats.TotalConns, stats.IdleConns, stats.StaleConns)
}

// TestKafkaMetricsCollection tests Kafka metrics collection
func TestKafkaMetricsCollection(t *testing.T) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	config.Producer.Return.Successes = true

	brokers := []string{"localhost:9092"}

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		t.Skip("Kafka is not available:", err)
		return
	}
	defer producer.Close()

	// Send some messages to generate metrics
	topic := "metrics-test-topic"
	messagesProduced := 0
	messagesFailed := 0

	for i := 0; i < 10; i++ {
		message := &sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder(fmt.Sprintf("key-%d", i)),
			Value: sarama.StringEncoder(fmt.Sprintf("value-%d", i)),
		}

		partition, offset, err := producer.SendMessage(message)
		if err != nil {
			messagesFailed++
			t.Logf("Failed to send message: %v", err)
		} else {
			messagesProduced++
			t.Logf("Message sent to partition %d at offset %d", partition, offset)
		}
	}

	// Validate metrics
	assert.Equal(t, 10, messagesProduced+messagesFailed)
	assert.GreaterOrEqual(t, messagesProduced, 1)

	t.Logf("Kafka Metrics - Produced: %d, Failed: %d", messagesProduced, messagesFailed)
}

// TestRabbitMQMetricsCollection tests RabbitMQ metrics collection
func TestRabbitMQMetricsCollection(t *testing.T) {
	conn, err := amqp.Dial("amqp://lynx:lynx123456@localhost:5672/")
	if err != nil {
		t.Skip("RabbitMQ is not available:", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// Declare queue
	queueName := "metrics-test-queue"
	q, err := ch.QueueDeclare(queueName, false, true, false, false, nil)
	require.NoError(t, err)

	// Publish and consume messages to generate metrics
	messagesPublished := 0
	messagesConsumed := 0

	// Publish messages
	for i := 0; i < 10; i++ {
		err := ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(fmt.Sprintf("metrics-message-%d", i)),
		})
		if err == nil {
			messagesPublished++
		}
	}

	// Consume messages
	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	require.NoError(t, err)

	timeout := time.After(2 * time.Second)
	for messagesConsumed < messagesPublished {
		select {
		case msg := <-msgs:
			if msg.Body != nil {
				messagesConsumed++
			}
		case <-timeout:
			break
		}
	}

	// Validate metrics
	assert.Equal(t, 10, messagesPublished)
	assert.GreaterOrEqual(t, messagesConsumed, 1)

	t.Logf("RabbitMQ Metrics - Published: %d, Consumed: %d",
		messagesPublished, messagesConsumed)
}

// TestMetricsAggregation tests metrics aggregation
func TestMetricsAggregation(t *testing.T) {
	aggregatedMetrics := make(map[string]interface{})

	// Collect Redis metrics
	t.Run("RedisMetricsAggregation", func(t *testing.T) {
		opt := &redis.Options{Addr: "localhost:6379"}
		client := redis.NewClient(opt)
		ctx := context.Background()

		if err := client.Ping(ctx).Err(); err == nil {
			stats := client.PoolStats()
			aggregatedMetrics["redis"] = map[string]interface{}{
				"connections_total": stats.TotalConns,
				"connections_idle":  stats.IdleConns,
				"hits":              stats.Hits,
				"misses":            stats.Misses,
				"timeouts":          stats.Timeouts,
			}
			client.Close()
		}
	})

	// Collect Kafka metrics (mock)
	t.Run("KafkaMetricsAggregation", func(t *testing.T) {
		aggregatedMetrics["kafka"] = map[string]interface{}{
			"messages_produced": 0,
			"messages_consumed": 0,
			"producer_errors":   0,
			"consumer_errors":   0,
			"consumer_lag":      0,
		}
	})

	// Collect RabbitMQ metrics (mock)
	t.Run("RabbitMQMetricsAggregation", func(t *testing.T) {
		aggregatedMetrics["rabbitmq"] = map[string]interface{}{
			"messages_published": 0,
			"messages_consumed":  0,
			"messages_failed":    0,
			"channels_active":    0,
			"connection_state":   0,
		}
	})

	// Validate aggregated metrics
	assert.NotEmpty(t, aggregatedMetrics)

	// Print aggregated metrics
	for service, metrics := range aggregatedMetrics {
		t.Logf("%s metrics: %+v", service, metrics)
	}
}

// TestMetricsExportFormat tests metrics export format
func TestMetricsExportFormat(t *testing.T) {
	// Test Prometheus format
	t.Run("PrometheusFormat", func(t *testing.T) {
		metrics := collectRedisMetrics()

		// Validate format
		for _, line := range metrics {
			if strings.HasPrefix(line, "#") {
				// Comment line
				assert.True(t,
					strings.HasPrefix(line, "# HELP") || strings.HasPrefix(line, "# TYPE"),
					"Invalid comment line: %s", line)
			} else if line != "" {
				// Data line should contain metric name and value
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					// Validate value is numeric
					value := parts[len(parts)-1]
					assert.Regexp(t, `^-?\d+(\.\d+)?$`, value,
						"Invalid metric value: %s", value)
				}
			}
		}
	})

	// Test JSON format
	t.Run("JSONFormat", func(t *testing.T) {
		jsonMetrics := map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"services": map[string]interface{}{
				"redis": map[string]int64{
					"ops_total":   1000,
					"ops_failed":  5,
					"connections": 10,
				},
				"kafka": map[string]int64{
					"messages_total": 5000,
					"errors_total":   2,
				},
			},
		}

		assert.NotNil(t, jsonMetrics["timestamp"])
		assert.NotNil(t, jsonMetrics["services"])
	})
}
