//go:build linux

package sniffer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/mdlayher/packet"
	"golang.org/x/sys/unix"
)

// Параметры AF_PACKET-сокета.
const (
	// frameSize — максимальный размер захватываемого кадра (включая VLAN-теги).
	frameSize = 65536
	// pollTimeout — период опроса сокета; позволяет быстро реагировать
	// на отмену контекста (ctx.Done).
	pollTimeout = 500 * time.Millisecond
)

// packetCapture — захватчик пакетов на AF_PACKET (Linux, без cgo).
//
// Использует github.com/mdlayher/packet — чистую реализацию
// на системных вызовах (socket(AF_PACKET), bind, recvfrom), без libpcap и cgo.
type packetCapture struct {
	logger *slog.Logger
	iface  string
	conn   *packet.Conn
}

// newPacketCapture создаёт AF_PACKET-сокет на указанном интерфейсе.
func newPacketCapture(logger *slog.Logger, iface string) (*packetCapture, error) {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, fmt.Errorf("sniffer: интерфейс %q: %w", iface, err)
	}
	// packet.Raw — SOCK_RAW: захват полных Ethernet-кадров (с заголовком).
	// unix.ETH_P_ALL — захват кадров всех протоколов (как afpacket.SocketRaw).
	conn, err := packet.Listen(ifi, packet.Raw, unix.ETH_P_ALL, &packet.Config{})
	if err != nil {
		return nil, fmt.Errorf("sniffer: создание AF_PACKET-сокета на %q: %w", iface, err)
	}
	return &packetCapture{logger: logger, iface: iface, conn: conn}, nil
}

// run захватывает пакеты и передаёт их обработчикам до отмены ctx.
// Блокируется, пока контекст не будет отменён либо не произойдёт ошибка.
func (c *packetCapture) run(ctx context.Context, handlers []PacketHandler) error {
	defer c.conn.Close()

	buf := make([]byte, frameSize)
	for {
		// Таймаут чтения позволяет периодически проверять отмену контекста.
		if err := c.conn.SetReadDeadline(time.Now().Add(pollTimeout)); err != nil {
			return fmt.Errorf("sniffer: установка таймаута чтения: %w", err)
		}

		n, _, err := c.conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				// Таймаут опроса: проверяем отмену контекста и продолжаем.
				select {
				case <-ctx.Done():
					return nil
				default:
					continue
				}
			}
			return fmt.Errorf("sniffer: чтение пакета: %w", err)
		}

		// Обработчики вызываются синхронно, поэтому буфер безопасно
		// переиспользовать: слайс frame валиден только на время вызова.
		frame := buf[:n]
		for _, h := range handlers {
			h.Handle(frame)
		}
	}
}
