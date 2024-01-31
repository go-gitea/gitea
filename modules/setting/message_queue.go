package setting

import (
	"fmt"
)

// MQConfig represents the configuration for a message queue
type MQConfig struct {
	SslPath       string `ini:"SSL_PATH" json:",omitempty"`
	TopicName     string `ini:"TOPIC_NAME" json:",omitempty"`
	ServerAddr    string `ini:"SERVER_ADDR" json:",omitempty"`
	MessageType   string `ini:"MESSAGE_TYPE" json:",omitempty"`
	ServerVersion string `ini:"SERVER_VERSION" json:",omitempty"`
}

// MQ represents the configuration of the message queue
var MQ MQConfig

// loadMQFrom loads message queue configuration from the given root configuration provider
func loadMQFrom(rootCfg ConfigProvider) error {
	sec, err := rootCfg.GetSection("message")
	if err != nil {
		return fmt.Errorf("failed to get 'message' section: %v", err)
	}

	if err := sec.MapTo(&MQ); err != nil {
		return fmt.Errorf("failed to map message queue settings: %v", err)
	}
	return nil
}
