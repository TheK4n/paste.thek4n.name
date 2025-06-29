package apikeys

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"

	"github.com/thek4n/paste.thek4n.name/internal/config"
	"github.com/thek4n/paste.thek4n.name/pkg/apikeys"
)

type Broker struct {
	Channel *amqp.Channel
}

func (b *Broker) SendAPIKeyUsageLog(apikey string, reason apikeys.UsageReason, fromIP string) error {
	a := &apikeys.APIKeyUsage{
		ApikeyId: apikey,
		Reason:   reason,
		FromIP:   fromIP,
	}
	data, err := proto.Marshal(a)
	if err != nil {
		return fmt.Errorf("can`t marshal record: %w", err)
	}

	err = b.Channel.Publish(
		config.APIKEYUSAGE_EXCHANGE,
		"",    // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/protobuf",
			Body:        data,
		},
	)
	if err != nil {
		return fmt.Errorf("can`t publish api_key_usage log: %w", err)
	}

	return nil
}

func InitBroker(connectURL string) (*Broker, error) {
	rabbitmqcon, err := amqp.Dial(connectURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	ch, err := rabbitmqcon.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create a rabbitmq channel: %w", err)
	}

	err = ch.ExchangeDeclare(
		config.APIKEYUSAGE_EXCHANGE,
		"direct", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create a rabbitmq exchange '%s': %w", config.APIKEYUSAGE_EXCHANGE, err)
	}

	return &Broker{
		Channel: ch,
	}, nil
}
