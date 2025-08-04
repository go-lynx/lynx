package kafka

import (
	"fmt"
)

// validateTopic 验证主题名称
func (k *KafkaClient) validateTopic(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic name cannot be empty")
	}

	// 检查主题名称长度
	if len(topic) > 249 {
		return fmt.Errorf("topic name too long, maximum length is 249 characters")
	}

	// 检查主题名称格式
	for _, char := range topic {
		if char < 32 || char > 126 {
			return fmt.Errorf("topic name contains invalid characters")
		}
	}

	return nil
}
