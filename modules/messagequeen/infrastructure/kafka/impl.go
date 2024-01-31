package kafka

import (
	"encoding/json"

	kfklib "github.com/opensourceways/kafka-lib/agent"
)

func Publish(topic string, v interface{}, header map[string]string) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return kfklib.Publish(topic, header, body)
}
