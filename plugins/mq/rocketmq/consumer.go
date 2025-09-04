package rocketmq

import (
	"context"
	"time"

	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/go-lynx/lynx/app/log"
)

// Subscribe subscribes to topics and sets message handler
func (r *Client) Subscribe(ctx context.Context, topics []string, handler MessageHandler) error {
	return r.SubscribeWith(ctx, r.defaultConsumer, topics, handler)
}

// SubscribeWith subscribes by consumer instance name
func (r *Client) SubscribeWith(ctx context.Context, consumerName string, topics []string, handler MessageHandler) error {
	start := time.Now()
	defer func() {
		r.metrics.RecordConsumerLatency(time.Since(start))
	}()

	// Validate parameters
	if len(topics) == 0 {
		return WrapError(ErrInvalidTopic, "no topics provided")
	}

	for _, topic := range topics {
		if err := validateTopic(topic); err != nil {
			return WrapError(err, "invalid topic: "+topic)
		}
	}

	if handler == nil {
		return WrapError(ErrConsumeMessageFailed, "message handler is nil")
	}

	// Get consumer
	consumer, err := r.GetConsumer(consumerName)
	if err != nil {
		return err
	}

	// Subscribe to topics
	err = consumer.Subscribe(topics[0], primitive.MessageSelector{}, func(ctx context.Context, msgs ...*primitive.MessageExt) (primitive.ConsumeResult, error) {
		for _, msg := range msgs {
			start := time.Now()

			// Call user handler
			if err := handler(ctx, msg); err != nil {
				r.metrics.IncrementConsumerMessagesFailed()
				log.Error("Failed to process RocketMQ message", "consumer", consumerName, "topic", msg.Topic, "error", err)
				return primitive.ConsumeRetryLater, err
			}

			r.metrics.RecordConsumerLatency(time.Since(start))
			r.metrics.IncrementConsumerMessagesReceived()
			log.Debug("Processed RocketMQ message", "consumer", consumerName, "topic", msg.Topic, "msgId", msg.MsgId)
		}
		return primitive.ConsumeSuccess, nil
	})

	if err != nil {
		log.Error("Failed to subscribe to RocketMQ topics", "consumer", consumerName, "topics", topics, "error", err)
		return WrapError(err, "failed to subscribe to topics")
	}

	// Start consumer
	if err := consumer.Start(); err != nil {
		log.Error("Failed to start RocketMQ consumer", "consumer", consumerName, "error", err)
		return WrapError(err, "failed to start consumer")
	}

	log.Info("Subscribed to RocketMQ topics", "consumer", consumerName, "topics", topics)
	return nil
}

// GetConsumer gets the underlying consumer client
func (r *Client) GetConsumer(name string) (primitive.PushConsumer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if name == "" {
		name = r.defaultConsumer
	}

	consumer, exists := r.consumers[name]
	if !exists {
		return nil, WrapError(ErrConsumerNotFound, "consumer not found: "+name)
	}

	return consumer, nil
}

// IsConsumerReady checks if the consumer is ready
func (r *Client) IsConsumerReady(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if name == "" {
		name = r.defaultConsumer
	}

	consumer, exists := r.consumers[name]
	if !exists {
		return false
	}

	// Check if consumer is running
	// Note: RocketMQ consumer doesn't have a direct "ready" method
	// We can check if it's not nil and assume it's ready if it was created successfully
	return consumer != nil
}
