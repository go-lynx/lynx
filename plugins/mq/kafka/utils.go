package kafka

import (
	"fmt"
)

// validateTopic validates topic name
func (k *Client) validateTopic(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic name cannot be empty")
	}

	// Check topic name length
	if len(topic) > 249 {
		return fmt.Errorf("topic name too long, maximum length is 249 characters")
	}

	// Check topic name format
	for _, char := range topic {
		if char < 32 || char > 126 {
			return fmt.Errorf("topic name contains invalid characters")
		}
	}

	return nil
}
