package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/IBM/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

// Define Prometheus metrics
var (
	// Redis metrics
	redisOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_redis_operations_total",
			Help: "Total number of Redis operations",
		},
		[]string{"operation"},
	)
	redisLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lynx_redis_operation_duration_seconds",
			Help:    "Redis operation latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	// Kafka metrics
	kafkaMessagesProduced = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_kafka_messages_produced_total",
			Help: "Total number of messages produced to Kafka",
		},
		[]string{"topic"},
	)
	kafkaMessagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_kafka_messages_consumed_total",
			Help: "Total number of messages consumed from Kafka",
		},
		[]string{"topic"},
	)
	kafkaProducerErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "lynx_kafka_producer_errors_total",
			Help: "Total number of Kafka producer errors",
		},
	)

	// RabbitMQ metrics
	rabbitMessagesPublished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_rabbitmq_messages_published_total",
			Help: "Total number of messages published to RabbitMQ",
		},
		[]string{"queue"},
	)
	rabbitMessagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_rabbitmq_messages_consumed_total",
			Help: "Total number of messages consumed from RabbitMQ",
		},
		[]string{"queue"},
	)
	rabbitConnectionState = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "lynx_rabbitmq_connection_state",
			Help: "RabbitMQ connection state (1=connected, 0=disconnected)",
		},
	)

	// Application metrics
	appHealthStatus = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "lynx_app_health_status",
			Help: "Overall application health status",
		},
	)
	appUptime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "lynx_app_uptime_seconds",
			Help: "Application uptime in seconds",
		},
	)
)

func init() {
	// Register all metrics
	prometheus.MustRegister(
		redisOpsTotal,
		redisLatency,
		kafkaMessagesProduced,
		kafkaMessagesConsumed,
		kafkaProducerErrors,
		rabbitMessagesPublished,
		rabbitMessagesConsumed,
		rabbitConnectionState,
		appHealthStatus,
		appUptime,
	)
}

type MetricsApp struct {
	redisClient   *redis.Client
	kafkaProducer sarama.SyncProducer
	rabbitConn    *amqp.Connection
	rabbitChannel *amqp.Channel
	startTime     time.Time
}

func NewMetricsApp() (*MetricsApp, error) {
	app := &MetricsApp{
		startTime: time.Now(),
	}

	// Initialize Redis client
	app.redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := app.redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis not available: %v", err)
	}

	// Initialize Kafka producer
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForLocal

	producer, err := sarama.NewSyncProducer([]string{"localhost:9092"}, config)
	if err != nil {
		log.Printf("Warning: Kafka not available: %v", err)
	} else {
		app.kafkaProducer = producer
	}

	// Initialize RabbitMQ connection
	conn, err := amqp.Dial("amqp://lynx:lynx123456@localhost:5672/")
	if err != nil {
		log.Printf("Warning: RabbitMQ not available: %v", err)
		rabbitConnectionState.Set(0)
	} else {
		app.rabbitConn = conn
		ch, err := conn.Channel()
		if err != nil {
			log.Printf("Warning: Failed to open RabbitMQ channel: %v", err)
		} else {
			app.rabbitChannel = ch
			rabbitConnectionState.Set(1)
		}
	}

	appHealthStatus.Set(1)
	return app, nil
}

func (app *MetricsApp) SimulateRedisOperations() {
	if app.redisClient == nil {
		return
	}

	ctx := context.Background()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// SET operation
		start := time.Now()
		key := fmt.Sprintf("test-key-%d", rand.Intn(100))
		err := app.redisClient.Set(ctx, key, "test-value", time.Minute).Err()
		duration := time.Since(start).Seconds()
		
		if err == nil {
			redisOpsTotal.WithLabelValues("set").Inc()
			redisLatency.WithLabelValues("set").Observe(duration)
		}

		// GET operation
		start = time.Now()
		_, err = app.redisClient.Get(ctx, key).Result()
		duration = time.Since(start).Seconds()
		
		if err == nil {
			redisOpsTotal.WithLabelValues("get").Inc()
			redisLatency.WithLabelValues("get").Observe(duration)
		}

		// DEL operation
		start = time.Now()
		app.redisClient.Del(ctx, key)
		duration = time.Since(start).Seconds()
		
		redisOpsTotal.WithLabelValues("del").Inc()
		redisLatency.WithLabelValues("del").Observe(duration)

		log.Printf("Redis operations completed: SET, GET, DEL for key %s", key)
	}
}

func (app *MetricsApp) SimulateKafkaOperations() {
	if app.kafkaProducer == nil {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	topic := "metrics-test-topic"
	
	for range ticker.C {
		for i := 0; i < 5; i++ {
			message := &sarama.ProducerMessage{
				Topic: topic,
				Key:   sarama.StringEncoder(fmt.Sprintf("key-%d", i)),
				Value: sarama.StringEncoder(fmt.Sprintf("message-%d-%d", time.Now().Unix(), i)),
			}

			partition, offset, err := app.kafkaProducer.SendMessage(message)
			if err != nil {
				kafkaProducerErrors.Inc()
				log.Printf("Kafka producer error: %v", err)
			} else {
				kafkaMessagesProduced.WithLabelValues(topic).Inc()
				log.Printf("Kafka message sent to partition %d at offset %d", partition, offset)
			}
		}
	}
}

func (app *MetricsApp) SimulateRabbitMQOperations() {
	if app.rabbitChannel == nil {
		return
	}

	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	queueName := "metrics-test-queue"
	
	// Declare queue
	q, err := app.rabbitChannel.QueueDeclare(
		queueName,
		false, // durable
		true,  // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		log.Printf("Failed to declare queue: %v", err)
		return
	}

	for range ticker.C {
		// Publish messages
		for i := 0; i < 3; i++ {
			body := fmt.Sprintf("rabbitmq-message-%d-%d", time.Now().Unix(), i)
			err := app.rabbitChannel.Publish(
				"",     // exchange
				q.Name, // routing key
				false,  // mandatory
				false,  // immediate
				amqp.Publishing{
					ContentType: "text/plain",
					Body:        []byte(body),
				},
			)
			if err != nil {
				log.Printf("RabbitMQ publish error: %v", err)
			} else {
				rabbitMessagesPublished.WithLabelValues(queueName).Inc()
				log.Printf("RabbitMQ message published: %s", body)
			}
		}

		// Consume messages
		msgs, err := app.rabbitChannel.Consume(
			q.Name,
			"",    // consumer
			true,  // auto-ack
			false, // exclusive
			false, // no-local
			false, // no-wait
			nil,   // args
		)
		if err != nil {
			log.Printf("Failed to consume messages: %v", err)
			continue
		}

		// Process a few messages
		consumed := 0
		timeout := time.After(1 * time.Second)
		for consumed < 3 {
			select {
			case msg := <-msgs:
				if msg.Body != nil {
					rabbitMessagesConsumed.WithLabelValues(queueName).Inc()
					consumed++
					log.Printf("RabbitMQ message consumed: %s", string(msg.Body))
				}
			case <-timeout:
				break
			}
		}
	}
}

func (app *MetricsApp) UpdateAppMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		uptime := time.Since(app.startTime).Seconds()
		appUptime.Set(uptime)
		
		// Check health of connections
		health := 1.0
		if app.redisClient != nil {
			if err := app.redisClient.Ping(context.Background()).Err(); err != nil {
				health = 0.5
			}
		}
		if app.rabbitConn != nil && app.rabbitConn.IsClosed() {
			health = 0.5
			rabbitConnectionState.Set(0)
		} else if app.rabbitConn != nil {
			rabbitConnectionState.Set(1)
		}
		
		appHealthStatus.Set(health)
		log.Printf("App metrics updated - Uptime: %.0fs, Health: %.1f", uptime, health)
	}
}

func (app *MetricsApp) Close() {
	if app.redisClient != nil {
		app.redisClient.Close()
	}
	if app.kafkaProducer != nil {
		app.kafkaProducer.Close()
	}
	if app.rabbitChannel != nil {
		app.rabbitChannel.Close()
	}
	if app.rabbitConn != nil {
		app.rabbitConn.Close()
	}
}

func main() {
	log.Println("Starting Lynx Metrics Test Application...")
	
	app, err := NewMetricsApp()
	if err != nil {
		log.Printf("Warning during initialization: %v", err)
	}
	defer app.Close()

	// Start simulation goroutines
	go app.SimulateRedisOperations()
	go app.SimulateKafkaOperations()
	go app.SimulateRabbitMQOperations()
	go app.UpdateAppMetrics()

	// Start metrics HTTP server
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK - Uptime: %.0fs\n", time.Since(app.startTime).Seconds())
	})

	log.Println("Metrics server starting on :8080...")
	log.Println("Access metrics at http://localhost:8080/metrics")
	log.Println("Access health at http://localhost:8080/health")
	
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Failed to start metrics server:", err)
	}
}