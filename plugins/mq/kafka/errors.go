package kafka

import "errors"

var (
	ErrProducerNotInitialized = errors.New("kafka producer not initialized")
	ErrConsumerNotInitialized = errors.New("kafka consumer not initialized")
)
