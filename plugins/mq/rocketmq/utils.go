package rocketmq

import (
	"strings"
	"time"
)

// validateTopic validates topic name
func validateTopic(topic string) error {
	if topic == "" {
		return ErrEmptyTopic
	}

	// Basic validation for RocketMQ topic naming
	if len(topic) > 255 {
		return WrapError(ErrInvalidTopic, "topic name too long")
	}

	// Check for invalid characters
	invalidChars := []string{"%", "&", "*", "+", "/", "\\", ":", "|", "<", ">", "?", " "}
	for _, char := range invalidChars {
		if strings.Contains(topic, char) {
			return WrapError(ErrInvalidTopic, "topic contains invalid character: "+char)
		}
	}

	return nil
}

// validateGroupName validates group name
func validateGroupName(groupName string) error {
	if groupName == "" {
		return ErrInvalidGroupName
	}

	// Basic validation for RocketMQ group naming
	if len(groupName) > 255 {
		return WrapError(ErrInvalidGroupName, "group name too long")
	}

	// Check for invalid characters
	invalidChars := []string{"%", "&", "*", "+", "/", "\\", ":", "|", "<", ">", "?", " "}
	for _, char := range invalidChars {
		if strings.Contains(groupName, char) {
			return WrapError(ErrInvalidGroupName, "group name contains invalid character: "+char)
		}
	}

	return nil
}

// validateConsumeModel validates consume model
func validateConsumeModel(model string) error {
	switch model {
	case ConsumeModelClustering, ConsumeModelBroadcast:
		return nil
	default:
		return WrapError(ErrInvalidConsumeModel, "invalid consume model: "+model)
	}
}

// validateConsumeOrder validates consume order
func validateConsumeOrder(order string) error {
	switch order {
	case ConsumeOrderConcurrent, ConsumeOrderOrderly:
		return nil
	default:
		return WrapError(ErrInvalidConsumeOrder, "invalid consume order: "+order)
	}
}

// parseDuration parses duration string with default fallback
func parseDuration(durationStr string, defaultDuration time.Duration) time.Duration {
	if durationStr == "" {
		return defaultDuration
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return defaultDuration
	}

	return duration
}

// isHealthy checks if the service is healthy based on error count and last check time
func isHealthy(errorCount int64, lastCheck time.Time, maxErrors int64, maxAge time.Duration) bool {
	if errorCount > maxErrors {
		return false
	}

	if time.Since(lastCheck) > maxAge {
		return false
	}

	return true
}
