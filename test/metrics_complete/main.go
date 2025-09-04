package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/elastic/go-elasticsearch/v8"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Prometheus metrics for all plugins
var (
	// Redis metrics
	redisOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_redis_operations_total",
			Help: "Total number of Redis operations",
		},
		[]string{"operation", "status"},
	)
	redisLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lynx_redis_operation_duration_seconds",
			Help:    "Redis operation latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
	redisConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "lynx_redis_connections_active",
			Help: "Number of active Redis connections",
		},
	)

	// Kafka metrics
	kafkaMessagesProduced = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_kafka_messages_produced_total",
			Help: "Total number of messages produced to Kafka",
		},
		[]string{"topic", "partition"},
	)
	kafkaMessagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_kafka_messages_consumed_total",
			Help: "Total number of messages consumed from Kafka",
		},
		[]string{"topic", "partition", "consumer_group"},
	)
	kafkaProducerErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_kafka_producer_errors_total",
			Help: "Total number of Kafka producer errors",
		},
		[]string{"topic"},
	)
	kafkaConsumerLag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynx_kafka_consumer_lag",
			Help: "Kafka consumer lag",
		},
		[]string{"topic", "partition", "consumer_group"},
	)

	// RabbitMQ metrics
	rabbitMessagesPublished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_rabbitmq_messages_published_total",
			Help: "Total number of messages published to RabbitMQ",
		},
		[]string{"exchange", "queue"},
	)
	rabbitMessagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_rabbitmq_messages_consumed_total",
			Help: "Total number of messages consumed from RabbitMQ",
		},
		[]string{"queue", "consumer_tag"},
	)
	rabbitConnectionState = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "lynx_rabbitmq_connection_state",
			Help: "RabbitMQ connection state (1=connected, 0=disconnected)",
		},
	)
	rabbitChannelsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "lynx_rabbitmq_channels_active",
			Help: "Number of active RabbitMQ channels",
		},
	)

	// MySQL metrics
	mysqlQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_mysql_queries_total",
			Help: "Total number of MySQL queries",
		},
		[]string{"operation", "status"},
	)
	mysqlQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lynx_mysql_query_duration_seconds",
			Help:    "MySQL query duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
	mysqlConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "lynx_mysql_connections_active",
			Help: "Number of active MySQL connections",
		},
	)

	// PostgreSQL metrics
	postgresQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_postgres_queries_total",
			Help: "Total number of PostgreSQL queries",
		},
		[]string{"operation", "status"},
	)
	postgresQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lynx_postgres_query_duration_seconds",
			Help:    "PostgreSQL query duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
	postgresConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "lynx_postgres_connections_active",
			Help: "Number of active PostgreSQL connections",
		},
	)

	// MongoDB metrics
	mongoOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_mongodb_operations_total",
			Help: "Total number of MongoDB operations",
		},
		[]string{"operation", "collection", "status"},
	)
	mongoOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lynx_mongodb_operation_duration_seconds",
			Help:    "MongoDB operation duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	// Elasticsearch metrics
	elasticOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lynx_elasticsearch_operations_total",
			Help: "Total number of Elasticsearch operations",
		},
		[]string{"operation", "index", "status"},
	)
	elasticOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lynx_elasticsearch_operation_duration_seconds",
			Help:    "Elasticsearch operation duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
	elasticClusterHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynx_elasticsearch_cluster_health",
			Help: "Elasticsearch cluster health (0=red, 1=yellow, 2=green)",
		},
		[]string{"cluster"},
	)

	// Application metrics
	appHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lynx_app_health_status",
			Help: "Application component health status",
		},
		[]string{"component"},
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
		// Redis
		redisOpsTotal,
		redisLatency,
		redisConnectionsActive,
		// Kafka
		kafkaMessagesProduced,
		kafkaMessagesConsumed,
		kafkaProducerErrors,
		kafkaConsumerLag,
		// RabbitMQ
		rabbitMessagesPublished,
		rabbitMessagesConsumed,
		rabbitConnectionState,
		rabbitChannelsActive,
		// MySQL
		mysqlQueriesTotal,
		mysqlQueryDuration,
		mysqlConnectionsActive,
		// PostgreSQL
		postgresQueriesTotal,
		postgresQueryDuration,
		postgresConnectionsActive,
		// MongoDB
		mongoOperationsTotal,
		mongoOperationDuration,
		// Elasticsearch
		elasticOperationsTotal,
		elasticOperationDuration,
		elasticClusterHealth,
		// Application
		appHealthStatus,
		appUptime,
	)
}

type CompleteMetricsApp struct {
	redisClient   *redis.Client
	kafkaProducer sarama.SyncProducer
	kafkaConsumer sarama.Consumer
	rabbitConn    *amqp.Connection
	rabbitChannel *amqp.Channel
	mysqlDB       *sql.DB
	postgresDB    *sql.DB
	mongoClient   *mongo.Client
	esClient      *elasticsearch.Client
	startTime     time.Time
	activeChannels int
}

func NewCompleteMetricsApp() (*CompleteMetricsApp, error) {
	app := &CompleteMetricsApp{
		startTime: time.Now(),
	}

	// Initialize Redis
	app.redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	ctx := context.Background()
	if err := app.redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis not available: %v", err)
		appHealthStatus.WithLabelValues("redis").Set(0)
	} else {
		appHealthStatus.WithLabelValues("redis").Set(1)
		log.Println("✅ Redis connected")
	}

	// Initialize Kafka
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForLocal
	config.Consumer.Return.Errors = true

	producer, err := sarama.NewSyncProducer([]string{"localhost:9092"}, config)
	if err != nil {
		log.Printf("Warning: Kafka producer not available: %v", err)
		appHealthStatus.WithLabelValues("kafka_producer").Set(0)
	} else {
		app.kafkaProducer = producer
		appHealthStatus.WithLabelValues("kafka_producer").Set(1)
		log.Println("✅ Kafka producer connected")
	}

	consumer, err := sarama.NewConsumer([]string{"localhost:9092"}, config)
	if err != nil {
		log.Printf("Warning: Kafka consumer not available: %v", err)
		appHealthStatus.WithLabelValues("kafka_consumer").Set(0)
	} else {
		app.kafkaConsumer = consumer
		appHealthStatus.WithLabelValues("kafka_consumer").Set(1)
		log.Println("✅ Kafka consumer connected")
	}

	// Initialize RabbitMQ
	conn, err := amqp.Dial("amqp://lynx:lynx123456@localhost:5672/")
	if err != nil {
		log.Printf("Warning: RabbitMQ not available: %v", err)
		rabbitConnectionState.Set(0)
		appHealthStatus.WithLabelValues("rabbitmq").Set(0)
	} else {
		app.rabbitConn = conn
		ch, err := conn.Channel()
		if err != nil {
			log.Printf("Warning: Failed to open RabbitMQ channel: %v", err)
		} else {
			app.rabbitChannel = ch
			app.activeChannels = 1
			rabbitConnectionState.Set(1)
			rabbitChannelsActive.Set(float64(app.activeChannels))
			appHealthStatus.WithLabelValues("rabbitmq").Set(1)
			log.Println("✅ RabbitMQ connected")
		}
	}

	// Initialize MySQL
	mysqlDB, err := sql.Open("mysql", "lynx:lynx123456@tcp(localhost:3306)/lynx_test")
	if err != nil {
		log.Printf("Warning: MySQL connection failed: %v", err)
		appHealthStatus.WithLabelValues("mysql").Set(0)
	} else {
		app.mysqlDB = mysqlDB
		app.mysqlDB.SetMaxOpenConns(10)
		app.mysqlDB.SetMaxIdleConns(5)
		if err := app.mysqlDB.Ping(); err != nil {
			log.Printf("Warning: MySQL ping failed: %v", err)
			appHealthStatus.WithLabelValues("mysql").Set(0)
		} else {
			appHealthStatus.WithLabelValues("mysql").Set(1)
			log.Println("✅ MySQL connected")
		}
	}

	// Initialize PostgreSQL
	postgresDB, err := sql.Open("postgres", "postgres://lynx:lynx123456@localhost:5432/lynx_test?sslmode=disable")
	if err != nil {
		log.Printf("Warning: PostgreSQL connection failed: %v", err)
		appHealthStatus.WithLabelValues("postgres").Set(0)
	} else {
		app.postgresDB = postgresDB
		app.postgresDB.SetMaxOpenConns(10)
		app.postgresDB.SetMaxIdleConns(5)
		if err := app.postgresDB.Ping(); err != nil {
			log.Printf("Warning: PostgreSQL ping failed: %v", err)
			appHealthStatus.WithLabelValues("postgres").Set(0)
		} else {
			appHealthStatus.WithLabelValues("postgres").Set(1)
			log.Println("✅ PostgreSQL connected")
		}
	}

	// Initialize MongoDB
	mongoOpts := options.Client().ApplyURI("mongodb://lynx:lynx123456@localhost:27017/")
	mongoClient, err := mongo.Connect(context.Background(), mongoOpts)
	if err != nil {
		log.Printf("Warning: MongoDB connection failed: %v", err)
		appHealthStatus.WithLabelValues("mongodb").Set(0)
	} else {
		app.mongoClient = mongoClient
		if err := mongoClient.Ping(context.Background(), nil); err != nil {
			log.Printf("Warning: MongoDB ping failed: %v", err)
			appHealthStatus.WithLabelValues("mongodb").Set(0)
		} else {
			appHealthStatus.WithLabelValues("mongodb").Set(1)
			log.Println("✅ MongoDB connected")
		}
	}

	// Initialize Elasticsearch
	esCfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}
	esClient, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		log.Printf("Warning: Elasticsearch connection failed: %v", err)
		appHealthStatus.WithLabelValues("elasticsearch").Set(0)
	} else {
		app.esClient = esClient
		res, err := esClient.Info()
		if err != nil {
			log.Printf("Warning: Elasticsearch info failed: %v", err)
			appHealthStatus.WithLabelValues("elasticsearch").Set(0)
		} else {
			res.Body.Close()
			appHealthStatus.WithLabelValues("elasticsearch").Set(1)
			elasticClusterHealth.WithLabelValues("local").Set(2) // green
			log.Println("✅ Elasticsearch connected")
		}
	}

	appHealthStatus.WithLabelValues("app").Set(1)
	return app, nil
}

// Simulation methods for each component
func (app *CompleteMetricsApp) SimulateRedis() {
	if app.redisClient == nil {
		return
	}

	ctx := context.Background()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// SET operation
		start := time.Now()
		key := fmt.Sprintf("test-key-%d", rand.Intn(100))
		err := app.redisClient.Set(ctx, key, fmt.Sprintf("value-%d", time.Now().Unix()), time.Minute).Err()
		duration := time.Since(start).Seconds()
		
		if err == nil {
			redisOpsTotal.WithLabelValues("set", "success").Inc()
		} else {
			redisOpsTotal.WithLabelValues("set", "error").Inc()
		}
		redisLatency.WithLabelValues("set").Observe(duration)

		// GET operation
		start = time.Now()
		val, err := app.redisClient.Get(ctx, key).Result()
		duration = time.Since(start).Seconds()
		
		if err == nil {
			redisOpsTotal.WithLabelValues("get", "success").Inc()
			log.Printf("Redis GET: key=%s, value=%s", key, val)
		} else {
			redisOpsTotal.WithLabelValues("get", "error").Inc()
		}
		redisLatency.WithLabelValues("get").Observe(duration)

		// Update connection stats
		stats := app.redisClient.PoolStats()
		redisConnectionsActive.Set(float64(stats.TotalConns))
	}
}

func (app *CompleteMetricsApp) SimulateKafka() {
	if app.kafkaProducer == nil || app.kafkaConsumer == nil {
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	topic := "metrics-test"
	
	for range ticker.C {
		// Produce messages
		for i := 0; i < 3; i++ {
			message := &sarama.ProducerMessage{
				Topic: topic,
				Key:   sarama.StringEncoder(fmt.Sprintf("key-%d", i)),
				Value: sarama.StringEncoder(fmt.Sprintf("kafka-msg-%d-%d", time.Now().Unix(), i)),
			}

			partition, offset, err := app.kafkaProducer.SendMessage(message)
			if err != nil {
				kafkaProducerErrors.WithLabelValues(topic).Inc()
				log.Printf("Kafka producer error: %v", err)
			} else {
				kafkaMessagesProduced.WithLabelValues(topic, fmt.Sprintf("%d", partition)).Inc()
				log.Printf("Kafka produced: topic=%s, partition=%d, offset=%d", topic, partition, offset)
			}
		}

		// Consume messages
		partitions, _ := app.kafkaConsumer.Partitions(topic)
		if len(partitions) > 0 {
			pc, err := app.kafkaConsumer.ConsumePartition(topic, partitions[0], sarama.OffsetNewest)
			if err == nil {
				select {
				case msg := <-pc.Messages():
					kafkaMessagesConsumed.WithLabelValues(topic, "0", "test-group").Inc()
					log.Printf("Kafka consumed: %s", string(msg.Value))
				case <-time.After(1 * time.Second):
				}
				pc.Close()
			}
		}
	}
}

func (app *CompleteMetricsApp) SimulateRabbitMQ() {
	if app.rabbitChannel == nil {
		return
	}

	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	queueName := "metrics-queue"
	
	// Declare queue
	q, err := app.rabbitChannel.QueueDeclare(queueName, false, true, false, false, nil)
	if err != nil {
		log.Printf("Failed to declare queue: %v", err)
		return
	}

	for range ticker.C {
		// Publish messages
		for i := 0; i < 2; i++ {
			body := fmt.Sprintf("rabbit-msg-%d-%d", time.Now().Unix(), i)
			err := app.rabbitChannel.Publish("", q.Name, false, false, amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(body),
			})
			if err != nil {
				log.Printf("RabbitMQ publish error: %v", err)
			} else {
				rabbitMessagesPublished.WithLabelValues("", queueName).Inc()
				log.Printf("RabbitMQ published: queue=%s, msg=%s", queueName, body)
			}
		}

		// Consume messages
		msgs, err := app.rabbitChannel.Consume(q.Name, "", true, false, false, false, nil)
		if err == nil {
			timeout := time.After(1 * time.Second)
			for i := 0; i < 2; i++ {
				select {
				case msg := <-msgs:
					rabbitMessagesConsumed.WithLabelValues(queueName, "consumer-1").Inc()
					log.Printf("RabbitMQ consumed: %s", string(msg.Body))
				case <-timeout:
					break
				}
			}
		}
	}
}

func (app *CompleteMetricsApp) SimulateMySQL() {
	if app.mysqlDB == nil {
		return
	}

	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	// Create table if not exists
	app.mysqlDB.Exec(`CREATE TABLE IF NOT EXISTS metrics_test (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(255),
		value INT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)

	for range ticker.C {
		// INSERT
		start := time.Now()
		result, err := app.mysqlDB.Exec("INSERT INTO metrics_test (name, value) VALUES (?, ?)", 
			fmt.Sprintf("test-%d", rand.Intn(100)), rand.Intn(1000))
		duration := time.Since(start).Seconds()
		
		if err == nil {
			mysqlQueriesTotal.WithLabelValues("insert", "success").Inc()
			id, _ := result.LastInsertId()
			log.Printf("MySQL INSERT: id=%d", id)
		} else {
			mysqlQueriesTotal.WithLabelValues("insert", "error").Inc()
		}
		mysqlQueryDuration.WithLabelValues("insert").Observe(duration)

		// SELECT
		start = time.Now()
		rows, err := app.mysqlDB.Query("SELECT COUNT(*) FROM metrics_test")
		duration = time.Since(start).Seconds()
		
		if err == nil {
			mysqlQueriesTotal.WithLabelValues("select", "success").Inc()
			if rows.Next() {
				var count int
				rows.Scan(&count)
				log.Printf("MySQL SELECT: count=%d", count)
			}
			rows.Close()
		} else {
			mysqlQueriesTotal.WithLabelValues("select", "error").Inc()
		}
		mysqlQueryDuration.WithLabelValues("select").Observe(duration)

		// Update connection stats
		stats := app.mysqlDB.Stats()
		mysqlConnectionsActive.Set(float64(stats.OpenConnections))
	}
}

func (app *CompleteMetricsApp) SimulatePostgreSQL() {
	if app.postgresDB == nil {
		return
	}

	ticker := time.NewTicker(7 * time.Second)
	defer ticker.Stop()

	// Create table if not exists
	app.postgresDB.Exec(`CREATE TABLE IF NOT EXISTS metrics_test (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255),
		value INT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)

	for range ticker.C {
		// INSERT
		start := time.Now()
		var id int
		err := app.postgresDB.QueryRow("INSERT INTO metrics_test (name, value) VALUES ($1, $2) RETURNING id",
			fmt.Sprintf("test-%d", rand.Intn(100)), rand.Intn(1000)).Scan(&id)
		duration := time.Since(start).Seconds()
		
		if err == nil {
			postgresQueriesTotal.WithLabelValues("insert", "success").Inc()
			log.Printf("PostgreSQL INSERT: id=%d", id)
		} else {
			postgresQueriesTotal.WithLabelValues("insert", "error").Inc()
		}
		postgresQueryDuration.WithLabelValues("insert").Observe(duration)

		// SELECT
		start = time.Now()
		rows, err := app.postgresDB.Query("SELECT COUNT(*) FROM metrics_test")
		duration = time.Since(start).Seconds()
		
		if err == nil {
			postgresQueriesTotal.WithLabelValues("select", "success").Inc()
			if rows.Next() {
				var count int
				rows.Scan(&count)
				log.Printf("PostgreSQL SELECT: count=%d", count)
			}
			rows.Close()
		} else {
			postgresQueriesTotal.WithLabelValues("select", "error").Inc()
		}
		postgresQueryDuration.WithLabelValues("select").Observe(duration)

		// Update connection stats
		stats := app.postgresDB.Stats()
		postgresConnectionsActive.Set(float64(stats.OpenConnections))
	}
}

func (app *CompleteMetricsApp) SimulateMongoDB() {
	if app.mongoClient == nil {
		return
	}

	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	collection := app.mongoClient.Database("lynx_test").Collection("metrics")

	for range ticker.C {
		ctx := context.Background()
		
		// INSERT
		start := time.Now()
		doc := map[string]interface{}{
			"name":  fmt.Sprintf("test-%d", rand.Intn(100)),
			"value": rand.Intn(1000),
			"timestamp": time.Now(),
		}
		result, err := collection.InsertOne(ctx, doc)
		duration := time.Since(start).Seconds()
		
		if err == nil {
			mongoOperationsTotal.WithLabelValues("insert", "metrics", "success").Inc()
			log.Printf("MongoDB INSERT: id=%v", result.InsertedID)
		} else {
			mongoOperationsTotal.WithLabelValues("insert", "metrics", "error").Inc()
		}
		mongoOperationDuration.WithLabelValues("insert").Observe(duration)

		// COUNT
		start = time.Now()
		count, err := collection.CountDocuments(ctx, map[string]interface{}{})
		duration = time.Since(start).Seconds()
		
		if err == nil {
			mongoOperationsTotal.WithLabelValues("count", "metrics", "success").Inc()
			log.Printf("MongoDB COUNT: total=%d", count)
		} else {
			mongoOperationsTotal.WithLabelValues("count", "metrics", "error").Inc()
		}
		mongoOperationDuration.WithLabelValues("count").Observe(duration)
	}
}

func (app *CompleteMetricsApp) SimulateElasticsearch() {
	if app.esClient == nil {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Index document
		start := time.Now()
		doc := fmt.Sprintf(`{"name":"test-%d","value":%d,"timestamp":"%s"}`,
			rand.Intn(100), rand.Intn(1000), time.Now().Format(time.RFC3339))
		
		res, err := app.esClient.Index(
			"metrics-index",
			strings.NewReader(doc),
		)
		duration := time.Since(start).Seconds()
		
		if err == nil {
			elasticOperationsTotal.WithLabelValues("index", "metrics-index", "success").Inc()
			res.Body.Close()
			log.Printf("Elasticsearch INDEX: index=metrics-index")
		} else {
			elasticOperationsTotal.WithLabelValues("index", "metrics-index", "error").Inc()
		}
		elasticOperationDuration.WithLabelValues("index").Observe(duration)

		// Search
		start = time.Now()
		query := `{"query": {"match_all": {}}}`
		res, err = app.esClient.Search(
			app.esClient.Search.WithIndex("metrics-index"),
			app.esClient.Search.WithBody(strings.NewReader(query)),
		)
		duration = time.Since(start).Seconds()
		
		if err == nil {
			elasticOperationsTotal.WithLabelValues("search", "metrics-index", "success").Inc()
			res.Body.Close()
			log.Printf("Elasticsearch SEARCH: index=metrics-index")
		} else {
			elasticOperationsTotal.WithLabelValues("search", "metrics-index", "error").Inc()
		}
		elasticOperationDuration.WithLabelValues("search").Observe(duration)
	}
}

func (app *CompleteMetricsApp) UpdateAppMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		uptime := time.Since(app.startTime).Seconds()
		appUptime.Set(uptime)
		log.Printf("📊 App uptime: %.0fs", uptime)
	}
}

func (app *CompleteMetricsApp) Close() {
	if app.redisClient != nil {
		app.redisClient.Close()
	}
	if app.kafkaProducer != nil {
		app.kafkaProducer.Close()
	}
	if app.kafkaConsumer != nil {
		app.kafkaConsumer.Close()
	}
	if app.rabbitChannel != nil {
		app.rabbitChannel.Close()
	}
	if app.rabbitConn != nil {
		app.rabbitConn.Close()
	}
	if app.mysqlDB != nil {
		app.mysqlDB.Close()
	}
	if app.postgresDB != nil {
		app.postgresDB.Close()
	}
	if app.mongoClient != nil {
		app.mongoClient.Disconnect(context.Background())
	}
}

func main() {
	log.Println("🚀 Starting Lynx Complete Metrics Application...")
	
	app, err := NewCompleteMetricsApp()
	if err != nil {
		log.Printf("Warning during initialization: %v", err)
	}
	defer app.Close()

	// Start all simulation goroutines
	go app.SimulateRedis()
	go app.SimulateKafka()
	go app.SimulateRabbitMQ()
	go app.SimulateMySQL()
	go app.SimulatePostgreSQL()
	go app.SimulateMongoDB()
	go app.SimulateElasticsearch()
	go app.UpdateAppMetrics()

	// Start metrics HTTP server
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK - Uptime: %.0fs\n", time.Since(app.startTime).Seconds())
	})

	log.Println("📊 Metrics server starting on :8080...")
	log.Println("🔗 Access metrics at http://localhost:8080/metrics")
	log.Println("🔗 Access health at http://localhost:8080/health")
	log.Println("🔗 Access Grafana at http://localhost:3000 (admin/lynx123456)")
	log.Println("🔗 Access Prometheus at http://localhost:9091")
	
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Failed to start metrics server:", err)
	}
}