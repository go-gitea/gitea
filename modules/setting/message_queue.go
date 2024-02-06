package setting

import "fmt"

const mqSectionName = "message"

// MQ represents the configuration of the message queue
var MQ MQConfig

// MQConfig represents the configuration for a message queue
type MQConfig struct {
	TopicName      string `ini:"TOPIC_NAME"          json:",omitempty"`
	ServerAddr     string `ini:"SERVER_ADDR"         json:",omitempty"`
	MessageType    string `ini:"MESSAGE_TYPE"        json:",omitempty"`
	ServerVersion  string `ini:"SERVER_VERSION"      json:",omitempty"`
	Certificate    string `ini:"CERTIFICATE"         json:",omitempty"`
	Username       string `ini:"USERNAME"            json:",omitempty"`
	Password       string `ini:"PASSWORD"            json:",omitempty"`
	Algorithm      string `ini:"ALGORITHM"           json:",omitempty"`
	SkipCertVerify bool   `ini:"SKIP_CERT_VERIFY"    json:",omitempty"`
}

// loadMQFrom loads message queue configuration from the given root configuration provider
func loadMQFrom(rootCfg ConfigProvider) error {
	sec, err := rootCfg.GetSection(mqSectionName)
	if err != nil {
		return fmt.Errorf("failed to get '%s' section: %v", mqSectionName, err)
	}

	if err := sec.MapTo(&MQ); err != nil {
		return fmt.Errorf("failed to map message queue settings: %v", err)
	}

	return nil
}
