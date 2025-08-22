package rocketmq

import (
	"errors"
	"fmt"
)

// Error definitions
var (
	// Configuration errors
	ErrInvalidConfiguration = errors.New("invalid rocketmq configuration")
	ErrMissingNameServer    = errors.New("name server addresses are required")
	ErrInvalidProducer      = errors.New("invalid producer configuration")
	ErrInvalidConsumer      = errors.New("invalid consumer configuration")

	// Connection errors
	ErrConnectionFailed  = errors.New("failed to connect to rocketmq")
	ErrConnectionTimeout = errors.New("connection timeout")
	ErrConnectionClosed  = errors.New("connection is closed")

	// Producer errors
	ErrProducerNotReady   = errors.New("producer is not ready")
	ErrProducerNotFound   = errors.New("producer not found")
	ErrInvalidTopic       = errors.New("invalid topic")
	ErrInvalidMessage     = errors.New("invalid message")
	ErrSendMessageFailed  = errors.New("failed to send message")
	ErrSendMessageTimeout = errors.New("send message timeout")

	// Consumer errors
	ErrConsumerNotReady     = errors.New("consumer is not ready")
	ErrConsumerNotFound     = errors.New("consumer not found")
	ErrSubscribeFailed      = errors.New("failed to subscribe to topics")
	ErrConsumeMessageFailed = errors.New("failed to consume message")

	// Health check errors
	ErrHealthCheckFailed = errors.New("health check failed")
	ErrUnhealthy         = errors.New("service is unhealthy")

	// Retry errors
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
	ErrRetryTimeout       = errors.New("retry timeout")

	// Validation errors
	ErrEmptyTopic          = errors.New("topic cannot be empty")
	ErrEmptyMessage        = errors.New("message cannot be empty")
	ErrInvalidGroupName    = errors.New("invalid group name")
	ErrInvalidConsumeModel = errors.New("invalid consume model")
	ErrInvalidConsumeOrder = errors.New("invalid consume order")
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
