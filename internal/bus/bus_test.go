package bus

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestPublishSubscribeRoundTrip проверяет доставку события подписчику
// и десериализацию JSON в контракт события.
func TestPublishSubscribeRoundTrip(t *testing.T) {
	b := New(testLogger(), DefaultConfig())
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := b.Subscribe(ctx, TopicDeviceDiscovered)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	want := DeviceDiscoveredEvent{
		MAC:       "aa:bb:cc:dd:ee:ff",
		IP:        "192.168.1.10",
		Source:    "arp",
		Timestamp: time.Now().UTC(),
	}
	if err := b.Publish(ctx, TopicDeviceDiscovered, want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case msg := <-sub:
		var got DeviceDiscoveredEvent
		if err := DecodePayload(msg, &got); err != nil {
			t.Fatalf("DecodePayload: %v", err)
		}
		if got.MAC != want.MAC || got.IP != want.IP || got.Source != want.Source {
			t.Fatalf("DecodePayload: got %+v, want %+v", got, want)
		}
		if topic := msg.Metadata.Get("topic"); topic != string(TopicDeviceDiscovered) {
			t.Fatalf("metadata topic = %q, want %q", topic, TopicDeviceDiscovered)
		}
		if !msg.Ack() {
			t.Fatal("Ack(): сообщение уже было подтверждено")
		}
	case <-ctx.Done():
		t.Fatal("событие не доставлено за отведённое время")
	}
}

// TestConsumeHandler проверяет обработку событий через Consume.
func TestConsumeHandler(t *testing.T) {
	b := New(testLogger(), DefaultConfig())
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	received := make(chan DeviceUpdatedEvent, 1)
	go func() {
		_ = b.Consume(ctx, TopicDeviceUpdated, func(_ context.Context, _ Topic, msg *message.Message) error {
			var ev DeviceUpdatedEvent
			if err := DecodePayload(msg, &ev); err != nil {
				return err
			}
			received <- ev
			return nil
		})
	}()

	want := DeviceUpdatedEvent{MAC: "aa:bb:cc:dd:ee:01", IP: "10.0.0.5", Vendor: "Cisco"}
	if err := b.Publish(context.Background(), TopicDeviceUpdated, want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case got := <-received:
		if got.MAC != want.MAC || got.IP != want.IP || got.Vendor != want.Vendor {
			t.Fatalf("получено %+v, ожидалось %+v", got, want)
		}
	case <-ctx.Done():
		t.Fatal("обработчик Consume не вызван за отведённое время")
	}
}

// TestPersistentDeliversToLateSubscriber проверяет, что при Persistent=true
// событие, опубликованное до подписки, доставляется подписчику позже.
// Это защищает от гонки «публикация раньше подписки» при старте воркеров.
func TestPersistentDeliversToLateSubscriber(t *testing.T) {
	b := New(testLogger(), DefaultConfig())
	defer b.Close()

	if err := b.Publish(context.Background(), TopicDeviceDeleted,
		DeviceDeletedEvent{MAC: "aa:bb:cc:dd:ee:02"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := b.Subscribe(ctx, TopicDeviceDeleted)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	select {
	case msg := <-sub:
		var got DeviceDeletedEvent
		if err := DecodePayload(msg, &got); err != nil {
			t.Fatalf("DecodePayload: %v", err)
		}
		if got.MAC != "aa:bb:cc:dd:ee:02" {
			t.Fatalf("получен MAC %q", got.MAC)
		}
		msg.Ack()
	case <-ctx.Done():
		t.Fatal("сохранённое событие не доставлено позднему подписчику")
	}
}

// TestTopicsAreIsolated проверяет, что события разных топиков
// не пересекаются.
func TestTopicsAreIsolated(t *testing.T) {
	b := New(testLogger(), DefaultConfig())
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	linkSub, err := b.Subscribe(ctx, TopicLinkChanged)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := b.Publish(ctx, TopicSyncRequest, SyncRequestEvent{Reason: "test"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// В топике link.changed не должно быть события sync.request.
	select {
	case msg := <-linkSub:
		t.Fatalf("получено постороннее событие: %s", string(msg.Payload))
	case <-time.After(200 * time.Millisecond):
		// ожидаемо: событие не доставлено в чужой топик
	}
}
