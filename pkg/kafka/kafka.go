package kafka

import (
	"context"
	"encoding/json"
	"github.com/IBM/sarama"
	"time"
)

type Config struct {
	Endpoints []string  `mapstructure:"endpoints"`
	Username  string    `mapstructure:"username"`
	Password  string    `mapstructure:"password"`
	TLS       TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	EnableTLS          bool   `mapstructure:"enableTLS"`
	CACert             string `mapstructure:"caCrt"`
	ClientCert         string `mapstructure:"clientCrt"`
	ClientKey          string `mapstructure:"clientKey"`
	ClientKeyPassword  string `mapstructure:"clientKeyPwd"`
	InsecureSkipVerify bool   `mapstructure:"insecureSkipVerify"`
}

type Kafka[T any] struct {
	sarama *sarama.Config
	cfg    *Config
}

type Producer[T any] struct {
	topic    string
	producer sarama.SyncProducer
}

type Consumer[T any] struct {
	handler func(context.Context, *T) error
}

func New[T any](cfg *Config) *Kafka[T] {
	kfa := &Kafka[T]{
		cfg: cfg,
	}
	kfa.sarama = sarama.NewConfig()
	kfa.sarama.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	kfa.sarama.Consumer.Offsets.Initial = sarama.OffsetNewest
	kfa.sarama.Producer.Retry.Max = 1
	kfa.sarama.Producer.RequiredAcks = sarama.WaitForAll
	kfa.sarama.Producer.Return.Successes = true
	// sasl认证
	if cfg.Username != "" && cfg.Password != "" {
		kfa.sarama.Net.SASL.Enable = true
		kfa.sarama.Net.SASL.User = cfg.Username
		kfa.sarama.Net.SASL.Password = cfg.Password
	}
	return kfa
}

func (k *Kafka[T]) NewConsumer(topic string, group string, handler func(context.Context, *T) error) (*Consumer[T], error) {
	c := &Consumer[T]{
		handler: handler,
	}
	var err error
	consumer, err := sarama.NewConsumerGroup(k.cfg.Endpoints, group, k.sarama)
	if err != nil {
		return c, err
	}
	go func() {
		for {
			if err := consumer.Consume(context.Background(), []string{topic}, c); err != nil {
				time.Sleep(time.Second * 10)
				continue
			}
		}
	}()
	return c, nil
}

func (k *Kafka[T]) NewProducer(topic string) (*Producer[T], error) {
	producer := &Producer[T]{
		topic: topic,
	}
	var err error
	producer.producer, err = sarama.NewSyncProducer(k.cfg.Endpoints, k.sarama)
	if err != nil {
		return producer, err
	}
	return producer, nil
}

func (p *Producer[T]) Send(obj *T, header map[string]string) error {
	value, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.ByteEncoder(value),
	}
	if header != nil {
		for k, v := range header {
			msg.Headers = append(msg.Headers, sarama.RecordHeader{
				Key:   []byte(k),
				Value: []byte(v),
			})
		}
	}
	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return err
	}
	return nil
}

func (c *Consumer[T]) Setup(sess sarama.ConsumerGroupSession) error {
	return nil
}

func (c *Consumer[T]) Cleanup(sess sarama.ConsumerGroupSession) error {
	return nil
}

func (c *Consumer[T]) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				continue
			}
			kv := make(map[string]string)
			for _, header := range message.Headers {
				kv[string(header.Key)] = string(header.Value)
			}
			ctx := context.Background()
			if len(kv) > 0 {
				for k, v := range kv {
					ctx = context.WithValue(ctx, k, v)
				}
			}
			obj := new(T)
			err := json.Unmarshal(message.Value, obj)
			if err == nil {
				_ = c.handler(ctx, obj)
			}
			sess.MarkMessage(message, "")
		case <-sess.Context().Done():
			return nil
		}
	}
}
