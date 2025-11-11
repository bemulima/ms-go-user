package broker

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	Publish(ctx context.Context, routingKey string, payload interface{}) error
	Close() error
}

type rabbitPublisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
	confirms <-chan amqp.Confirmation
	mu       sync.Mutex
}

func NewRabbitMQPublisher(url, exchange string) (Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	return &rabbitPublisher{conn: conn, channel: ch, exchange: exchange, confirms: confirms}, nil
}

func (p *rabbitPublisher) Publish(ctx context.Context, routingKey string, payload interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := p.channel.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	}); err != nil {
		return err
	}
	select {
	case conf, ok := <-p.confirms:
		if !ok || !conf.Ack {
			return amqp.ErrClosed
		}
		return nil
	case <-ctx.Done():
		err := ctx.Err()
		if conf, ok := <-p.confirms; !ok || !conf.Ack {
			return amqp.ErrClosed
		}
		return err
	}
}

func (p *rabbitPublisher) Close() error {
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}
