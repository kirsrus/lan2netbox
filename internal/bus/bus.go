// Package bus реализует внутреннюю шину событий приложения.
//
// Шина построена на watermill с транспортом gochannel (in-memory, без
// внешних зависимостей). Компоненты обмениваются событиями через строго
// типизированные топики (константы Topic) и контракты событий (events.go).
//
// Карта потоков (из плана разработки):
//
//	device.discovered — sniffer → probe, web, zabbix
//	device.updated    — sniffer, probe → netbox, web, zabbix
//	device.deleted    — sniffer → netbox, web, zabbix
//	link.changed      — probe → netbox, web, zabbix
//	sync.request      — web → netbox
package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
)

// Topic — имя топика шины событий.
type Topic string

// Топики шины. Издатели и подписчики описаны в комментарии к пакету.
const (
	// TopicDeviceDiscovered — обнаружено новое устройство (MAC, IP, источник).
	TopicDeviceDiscovered Topic = "device.discovered"
	// TopicDeviceUpdated — запись устройства обновлена.
	TopicDeviceUpdated Topic = "device.updated"
	// TopicDeviceDeleted — устройство пропало из сети.
	TopicDeviceDeleted Topic = "device.deleted"
	// TopicLinkChanged — изменились связи устройство-коммутатор.
	TopicLinkChanged Topic = "link.changed"
	// TopicSyncRequest — принудительная синхронизация с NetBox (ручной запуск).
	TopicSyncRequest Topic = "sync.request"
)

// String возвращает строковое представление топика.
func (t Topic) String() string { return string(t) }

// AllTopics возвращает все топики шины (для логирования и диагностики).
func AllTopics() []Topic {
	return []Topic{
		TopicDeviceDiscovered,
		TopicDeviceUpdated,
		TopicDeviceDeleted,
		TopicLinkChanged,
		TopicSyncRequest,
	}
}

// Config — параметры внутренней шины (обёртка над gochannel.Config).
type Config struct {
	// OutputChannelBuffer — размер буфера канала доставки сообщений.
	OutputChannelBuffer int
	// BlockPublishUntilSubscriberAck — блокировать публикацию до
	// подтверждения (ACK) подписчика.
	BlockPublishUntilSubscriberAck bool
	// Persistent — хранить сообщения в памяти до подключения подписчика.
	// Рекомендуется включать, чтобы события не терялись при гонке
	// «публикация раньше подписки» (подписчики стартуют асинхронно).
	Persistent bool
}

// DefaultConfig возвращает конфигурацию шины по умолчанию.
func DefaultConfig() Config {
	return Config{
		OutputChannelBuffer:            1024,
		BlockPublishUntilSubscriberAck: false,
		Persistent:                     true,
	}
}

// Handler обрабатывает одно событие из топика.
type Handler func(ctx context.Context, topic Topic, msg *message.Message) error

// Bus — шина событий приложения.
type Bus struct {
	logger *slog.Logger
	pubSub *gochannel.GoChannel
}

// New создаёт шину событий с указанной конфигурацией.
func New(logger *slog.Logger, cfg Config) *Bus {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.OutputChannelBuffer <= 0 {
		cfg.OutputChannelBuffer = 1024
	}
	opts := gochannel.Config{
		OutputChannelBuffer:            int64(cfg.OutputChannelBuffer),
		BlockPublishUntilSubscriberAck: cfg.BlockPublishUntilSubscriberAck,
		Persistent:                     cfg.Persistent,
		// Контекст сообщения сохраняется при доставке подписчику.
		PreserveContext: true,
	}
	return &Bus{
		logger: logger,
		pubSub: gochannel.NewGoChannel(opts, slogAdapter{logger: logger}),
	}
}

// Publish сериализует payload в JSON и публикует событие в топик.
// Контекст сообщения сохраняется для подписчиков (см. Message.Context).
func (b *Bus) Publish(ctx context.Context, topic Topic, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("bus: маршалинг события %s: %w", topic, err)
	}
	msg := message.NewMessageWithContext(ctx, watermill.NewUUID(), data)
	msg.Metadata.Set("topic", topic.String())
	if err := b.pubSub.Publish(topic.String(), msg); err != nil {
		return fmt.Errorf("bus: публикация события %s: %w", topic, err)
	}
	return nil
}

// Subscribe подписывается на топик и возвращает канал событий.
// Канал закрывается при отмене ctx либо закрытии шины.
func (b *Bus) Subscribe(ctx context.Context, topic Topic) (<-chan *message.Message, error) {
	ch, err := b.pubSub.Subscribe(ctx, topic.String())
	if err != nil {
		return nil, fmt.Errorf("bus: подписка на топик %s: %w", topic, err)
	}
	return ch, nil
}

// Consume подписывается на топик и обрабатывает события до отмены ctx.
// Ошибка обработчика не прерывает цикл: событие логируется и помечается
// как ACK (доставлено).
func (b *Bus) Consume(ctx context.Context, topic Topic, h Handler) error {
	ch, err := b.Subscribe(ctx, topic)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := h(ctx, topic, msg); err != nil {
				b.logger.Error("bus: ошибка обработки события",
					"topic", topic.String(),
					"message_id", msg.UUID,
					"error", err,
				)
			}
			if !msg.Ack() {
				b.logger.Warn("bus: сообщение уже было подтверждено",
					"topic", topic.String(),
					"message_id", msg.UUID,
				)
			}
		}
	}
}

// ConsumeTopics подписывается на несколько топиков и обрабатывает события
// единым обработчиком. Блокируется до отмены ctx либо ошибки подписки.
func (b *Bus) ConsumeTopics(ctx context.Context, topics []Topic, h Handler) error {
	if len(topics) == 0 {
		<-ctx.Done()
		return nil
	}
	errCh := make(chan error, len(topics))
	for _, t := range topics {
		t := t
		go func() {
			errCh <- b.Consume(ctx, t, h)
		}()
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

// LogHandler возвращает обработчик-заглушку, логирующий полученные события.
// Используется на этапе разработки, пока реальная обработка не реализована.
func (b *Bus) LogHandler(component string) Handler {
	return func(_ context.Context, topic Topic, msg *message.Message) error {
		b.logger.Info("получено событие",
			"component", component,
			"topic", topic.String(),
			"message_id", msg.UUID,
			"payload", string(msg.Payload),
		)
		return nil
	}
}

// DecodePayload десериализует JSON-нагрузку события в dst.
func DecodePayload(msg *message.Message, dst any) error {
	if err := json.Unmarshal(msg.Payload, dst); err != nil {
		return fmt.Errorf("bus: десериализация события %q: %w", msg.Metadata.Get("topic"), err)
	}
	return nil
}

// Close закрывает шину и все подписки.
func (b *Bus) Close() error {
	if err := b.pubSub.Close(); err != nil {
		return fmt.Errorf("bus: закрытие шины: %w", err)
	}
	return nil
}

// slogAdapter адаптирует *slog.Logger к интерфейсу watermill.LoggerAdapter,
// чтобы логи водяной мельницы попадали в единую систему логирования.
type slogAdapter struct {
	logger *slog.Logger
}

func fieldsToArgs(fields watermill.LogFields) []any {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return args
}

func (a slogAdapter) Error(msg string, err error, fields watermill.LogFields) {
	a.logger.Error(msg, append(fieldsToArgs(fields), "error", err)...)
}

func (a slogAdapter) Info(msg string, fields watermill.LogFields) {
	a.logger.Info(msg, fieldsToArgs(fields)...)
}

func (a slogAdapter) Debug(msg string, fields watermill.LogFields) {
	a.logger.Debug(msg, fieldsToArgs(fields)...)
}

func (a slogAdapter) Trace(msg string, fields watermill.LogFields) {
	a.logger.Debug("[trace] "+msg, fieldsToArgs(fields)...)
}

func (a slogAdapter) With(fields watermill.LogFields) watermill.LoggerAdapter {
	return slogAdapter{logger: a.logger.With(fieldsToArgs(fields)...)}
}
