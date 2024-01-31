package messagequeen

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// Init the message queen, (ex: ActiveMQ、RocketMQ、RabbitMQ、Kafka)
func Init() (err error) {
	log.Info("Initialising message queen with type: %s", setting.MQ.MessageType)
	return newKafkaMessageQueue(setting.MQ)
}
