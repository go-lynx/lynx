package rocketmq

import (
	"context"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/go-lynx/lynx/app/log"
)

// SendMessage sends a single message to the specified topic
func (r *Client) SendMessage(ctx context.Context, topic string, body []byte) error {
	return r.SendMessageWith(ctx, r.defaultProducer, topic, body)
}

// SendMessageSync sends a message synchronously
func (r *Client) SendMessageSync(ctx context.Context, topic string, body []byte) (*primitive.SendResult, error) {
	return r.SendMessageSyncWith(ctx, r.defaultProducer, topic, body)
}

// SendMessageAsync sends a message asynchronously
func (r *Client) SendMessageAsync(ctx context.Context, topic string, body []byte) error {
	return r.SendMessageAsyncWith(ctx, r.defaultProducer, topic, body)
}

// SendMessageWith sends a message by producer instance name
func (r *Client) SendMessageWith(ctx context.Context, producerName, topic string, body []byte) error {
	start := time.Now()
	defer func() {
		r.metrics.RecordProducerLatency(time.Since(start))
	}()

	// Validate parameters
	if err := validateTopic(topic); err != nil {
		r.metrics.IncrementProducerMessagesFailed()
		return WrapError(err, "invalid topic")
	}

	if len(body) == 0 {
		r.metrics.IncrementProducerMessagesFailed()
		return ErrEmptyMessage
	}

	// Get producer
	producer, err := r.GetProducer(producerName)
	if err != nil {
		r.metrics.IncrementProducerMessagesFailed()
		return err
	}

	// Create message
	msg := primitive.NewMessage(topic, body)

	// Send message with retry
	err = r.retryHandler.DoWithRetry(ctx, func() error {
		_, err := producer.SendSync(ctx, msg)
		return err
	})

	if err != nil {
		r.metrics.IncrementProducerMessagesFailed()
		log.Error("Failed to send RocketMQ message", "producer", producerName, "topic", topic, "error", err)
		return WrapError(err, "failed to send message")
	}

	r.metrics.IncrementProducerMessagesSent()
	log.Debug("Sent RocketMQ message", "producer", producerName, "topic", topic)
	return nil
}

// SendMessageSyncWith sends a message synchronously by producer instance name
func (r *Client) SendMessageSyncWith(ctx context.Context, producerName, topic string, body []byte) (*primitive.SendResult, error) {
	start := time.Now()
	defer func() {
		r.metrics.RecordProducerLatency(time.Since(start))
	}()

	// Validate parameters
	if err := validateTopic(topic); err != nil {
		r.metrics.IncrementProducerMessagesFailed()
		return nil, WrapError(err, "invalid topic")
	}

	if len(body) == 0 {
		r.metrics.IncrementProducerMessagesFailed()
		return nil, ErrEmptyMessage
	}

	// Get producer
	producer, err := r.GetProducer(producerName)
	if err != nil {
		r.metrics.IncrementProducerMessagesFailed()
		return nil, err
	}

	// Create message
	msg := primitive.NewMessage(topic, body)

	// Send message with retry
	var result *primitive.SendResult
	err = r.retryHandler.DoWithRetry(ctx, func() error {
		var sendErr error
		result, sendErr = producer.SendSync(ctx, msg)
		return sendErr
	})

	if err != nil {
		r.metrics.IncrementProducerMessagesFailed()
		log.Error("Failed to send RocketMQ message sync", "producer", producerName, "topic", topic, "error", err)
		return nil, WrapError(err, "failed to send message")
	}

	r.metrics.IncrementProducerMessagesSent()
	log.Debug("Sent RocketMQ message sync", "producer", producerName, "topic", topic, "msgId", result.MsgID)
	return result, nil
}

// SendMessageAsyncWith sends a message asynchronously by producer instance name
func (r *Client) SendMessageAsyncWith(ctx context.Context, producerName, topic string, body []byte) error {
	start := time.Now()
	defer func() {
		r.metrics.RecordProducerLatency(time.Since(start))
	}()

	// Validate parameters
	if err := validateTopic(topic); err != nil {
		r.metrics.IncrementProducerMessagesFailed()
		return WrapError(err, "invalid topic")
	}

	if len(body) == 0 {
		r.metrics.IncrementProducerMessagesFailed()
		return ErrEmptyMessage
	}

	// Get producer
	producer, err := r.GetProducer(producerName)
	if err != nil {
		r.metrics.IncrementProducerMessagesFailed()
		return err
	}

	// Create message
	msg := primitive.NewMessage(topic, body)

	// Send message asynchronously
	err = producer.SendAsync(ctx, func(ctx context.Context, result *primitive.SendResult, err error) {
		if err != nil {
			r.metrics.IncrementProducerMessagesFailed()
			log.Error("Failed to send RocketMQ message async", "producer", producerName, "topic", topic, "error", err)
		} else {
			r.metrics.IncrementProducerMessagesSent()
			log.Debug("Sent RocketMQ message async", "producer", producerName, "topic", topic, "msgId", result.MsgID)
		}
	}, msg)

	if err != nil {
		r.metrics.IncrementProducerMessagesFailed()
		log.Error("Failed to send RocketMQ message async", "producer", producerName, "topic", topic, "error", err)
		return WrapError(err, "failed to send message async")
	}

	return nil
}

// GetProducer gets the underlying producer client
func (r *Client) GetProducer(name string) (rocketmq.Producer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if name == "" {
		name = r.defaultProducer
	}

	producer, exists := r.producers[name]
	if !exists {
		return nil, WrapError(ErrProducerNotFound, "producer not found: "+name)
	}

	return producer, nil
}

// IsProducerReady checks if the producer is ready
func (r *Client) IsProducerReady(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if name == "" {
		name = r.defaultProducer
	}

	producer, exists := r.producers[name]
	if !exists {
		return false
	}

	// Check if producer is running
	// Note: RocketMQ producer doesn't have a direct "ready" method
	// We can check if it's not nil and assume it's ready if it was created successfully
	return producer != nil
}
