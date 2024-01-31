package messagequeen

import (
	"github.com/sirupsen/logrus"

	kfklib "github.com/opensourceways/kafka-lib/agent"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	queueName = "gitea-kafka-queue"
)

func retriveConfig(cfg setting.MQConfig) kfklib.Config {
	kafkaAddr := cfg.ServerAddr
	kafkaVer := cfg.ServerVersion

	// Check if KAFKA_VER is set in the environment
	if kafkaVer == "" {
		log.Fatal("KAFKA_VER is not set in the environment. " +
			"It's crucial to set this to avoid protocol version mismatches and ensure backward compatibility.")
	}

	return kfklib.Config{
		Address:        kafkaAddr,
		Version:        kafkaVer,
		SkipCertVerify: true,
	}
}

// newKafkaMessageQueue sets up a new Kafka message queue
func newKafkaMessageQueue(cfg setting.MQConfig) error {
	var localConfig = retriveConfig(cfg)
	mqLog := logrus.NewEntry(logrus.StandardLogger())
	return kfklib.Init(&localConfig, mqLog, nil, queueName, true)
}
