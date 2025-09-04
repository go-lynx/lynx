package rabbitmq

import (
	"errors"
	"fmt"
)

// Error definitions
var (
	// Configuration errors
	ErrInvalidConfiguration = errors.New("invalid rabbitmq configuration")
	ErrMissingURLs          = errors.New("rabbitmq server URLs are required")
	ErrInvalidProducer      = errors.New("invalid producer configuration")
	ErrInvalidConsumer      = errors.New("invalid consumer configuration")

	// Connection errors
	ErrConnectionFailed  = errors.New("failed to connect to rabbitmq")
	ErrConnectionTimeout = errors.New("connection timeout")
	ErrConnectionClosed  = errors.New("connection is closed")
	ErrChannelFailed     = errors.New("failed to create channel")

	// Producer errors
	ErrProducerNotReady     = errors.New("producer is not ready")
	ErrProducerNotFound     = errors.New("producer not found")
	ErrInvalidExchange      = errors.New("invalid exchange")
	ErrInvalidRoutingKey    = errors.New("invalid routing key")
	ErrInvalidMessage       = errors.New("invalid message")
	ErrPublishMessageFailed = errors.New("failed to publish message")
	ErrPublishTimeout       = errors.New("publish timeout")

	// Consumer errors
	ErrConsumerNotReady     = errors.New("consumer is not ready")
	ErrConsumerNotFound     = errors.New("consumer not found")
	ErrInvalidQueue         = errors.New("invalid queue")
	ErrSubscribeFailed      = errors.New("failed to subscribe to queue")
	ErrConsumeMessageFailed = errors.New("failed to consume message")

	// Health check errors
	ErrHealthCheckFailed = errors.New("health check failed")
	ErrUnhealthy         = errors.New("service is unhealthy")

	// Retry errors
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
	ErrRetryTimeout       = errors.New("retry timeout")

	// Validation errors
	ErrEmptyExchange       = errors.New("exchange cannot be empty")
	ErrEmptyQueue          = errors.New("queue cannot be empty")
	ErrEmptyMessage        = errors.New("message cannot be empty")
	ErrInvalidExchangeType = errors.New("invalid exchange type")
	ErrInvalidVirtualHost  = errors.New("invalid virtual host")
)

// Error wrapper with context
type ErrorWithContext struct {
	Err     error
	Context string
}

func (e *ErrorWithContext) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("%s: %v", e.Context, e.Err)
	}
	return e.Err.Error()
}

func (e *ErrorWithContext) Unwrap() error {
	return e.Err
}

// WrapError wraps an error with context
func WrapError(err error, context string) error {
	return &ErrorWithContext{
		Err:     err,
		Context: context,
	}
}

// IsError checks if error is of specific type
func IsError(err error, target error) bool {
	return errors.Is(err, target)
}
