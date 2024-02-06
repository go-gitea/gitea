package messagequeue

import (
	"encoding/json"

	kfklib "github.com/opensourceways/kafka-lib/agent"
	"github.com/sirupsen/logrus"

	"code.gitea.io/gitea/modules/setting"
)

const queueName = "gitea-kafka-queue"

func Publish(topic string, v interface{}, header map[string]string) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return kfklib.Publish(topic, header, body)
}

func retriveConfig(cfg setting.MQConfig) kfklib.Config {
	return kfklib.Config{
		Address:        cfg.ServerAddr,
		Version:        cfg.ServerVersion,
		SkipCertVerify: cfg.SkipCertVerify,
		Username:       cfg.Username,
		Password:       cfg.Password,
		MQCert:         cfg.Certificate,
	}
}

// newKafkaMessageQueue sets up a new Kafka message queue
func newKafkaMessageQueue(cfg setting.MQConfig) error {
	v := retriveConfig(cfg)

	mqLog := logrus.NewEntry(logrus.StandardLogger())

	return kfklib.Init(&v, mqLog, nil, queueName, true)
}
