package rabbitmq

import (
	"strings"
	"time"
)

// validateExchange validates exchange name
func validateExchange(exchange string) error {
	if exchange == "" {
		return ErrEmptyExchange
	}

	// Basic validation for RabbitMQ exchange naming
	if len(exchange) > 255 {
		return WrapError(ErrInvalidExchange, "exchange name too long")
	}

	// Check for invalid characters
	invalidChars := []string{"%", "&", "*", "+", "/", "\\", ":", "|", "<", ">", "?", " "}
	for _, char := range invalidChars {
		if strings.Contains(exchange, char) {
			return WrapError(ErrInvalidExchange, "exchange contains invalid character: "+char)
		}
	}

	return nil
}

// validateQueue validates queue name
func validateQueue(queue string) error {
	if queue == "" {
		return ErrEmptyQueue
	}

	// Basic validation for RabbitMQ queue naming
	if len(queue) > 255 {
		return WrapError(ErrInvalidQueue, "queue name too long")
	}

	// Check for invalid characters
	invalidChars := []string{"%", "&", "*", "+", "/", "\\", ":", "|", "<", ">", "?", " "}
	for _, char := range invalidChars {
		if strings.Contains(queue, char) {
			return WrapError(ErrInvalidQueue, "queue contains invalid character: "+char)
		}
	}

	return nil
}

// validateExchangeType validates exchange type
func validateExchangeType(exchangeType string) error {
	switch exchangeType {
	case ExchangeTypeDirect, ExchangeTypeFanout, ExchangeTypeTopic, ExchangeTypeHeaders:
		return nil
	default:
		return WrapError(ErrInvalidExchangeType, "invalid exchange type: "+exchangeType)
	}
}

// validateVirtualHost validates virtual host
func validateVirtualHost(vhost string) error {
	if vhost == "" {
		return ErrInvalidVirtualHost
	}

	// Basic validation for RabbitMQ virtual host naming
	if len(vhost) > 255 {
		return WrapError(ErrInvalidVirtualHost, "virtual host name too long")
	}

	// Check for invalid characters
	invalidChars := []string{"%", "&", "*", "+", "/", "\\", ":", "|", "<", ">", "?", " "}
	for _, char := range invalidChars {
		if strings.Contains(vhost, char) {
			return WrapError(ErrInvalidVirtualHost, "virtual host contains invalid character: "+char)
		}
	}

	return nil
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

// buildAMQPURL builds AMQP URL from components
func buildAMQPURL(username, password, host, port, vhost string) string {
	if username == "" {
		username = "guest"
	}
	if password == "" {
		password = "guest"
	}
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5672"
	}
	if vhost == "" {
		vhost = "/"
	}

	return "amqp://" + username + ":" + password + "@" + host + ":" + port + "/" + vhost
}
