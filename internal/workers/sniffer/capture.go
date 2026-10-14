//go:build linux

package sniffer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"
)

// Параметры AF_PACKET-сокета.
const (
	// frameSize — максимальный размер захватываемого кадра (включая VLAN-теги).
	frameSize = 65536
	// blockSize — размер блока памяти кольцевого буфера (mmap).
	blockSize = 1 << 20 // 1 MiB
	// numBlocks — количество блоков кольцевого буфера.
	numBlocks = 128
	// pollTimeout — период опроса сокета; позволяет быстро реагировать
	// на отмену контекста (ctx.Done).
	pollTimeout = 500 * time.Millisecond
)

// packetCapture — захватчик пакетов на AF_PACKET (Linux, без cgo).
//
// Использует github.com/google/gopacket/afpacket — чистую реализацию
// на системных вызовах (socket(AF_PACKET), mmap, poll), без libpcap и cgo.
type packetCapture struct {
	logger *slog.Logger
	iface  string
	handle *afpacket.TPacket
}

// newPacketCapture создаёт AF_PACKET-сокет на указанном интерфейсе.
func newPacketCapture(logger *slog.Logger, iface string) (*packetCapture, error) {
	handle, err := afpacket.NewTPacket(
		afpacket.OptInterface(iface),
		afpacket.OptFrameSize(frameSize),
		afpacket.OptBlockSize(blockSize),
		afpacket.OptNumBlocks(numBlocks),
		afpacket.OptPollTimeout(pollTimeout),
		afpacket.SocketRaw,
		afpacket.TPacketVersion3,
	)
	if err != nil {
		return nil, fmt.Errorf("sniffer: создание AF_PACKET-сокета на %q: %w", iface, err)
	}
	return &packetCapture{logger: logger, iface: iface, handle: handle}, nil
}

// run захватывает пакеты и передаёт их обработчикам до отмены ctx.
// Блокируется, пока контекст не будет отменён либо не произойдёт ошибка.
func (c *packetCapture) run(ctx context.Context, handlers []PacketHandler) error {
	defer c.handle.Close()

	buf := make([]byte, frameSize)
	for {
		ci, err := c.handle.ReadPacketDataTo(buf)
		if err != nil {
			if errors.Is(err, afpacket.ErrTimeout) {
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

		// Lazy: слои декодируются по требованию (быстрее, если обработчики
		// смотрят не все слои). NoCopy: данные не копируются — обработчики
		// вызываются синхронно, поэтому буфер безопасно переиспользовать.
		pkt := gopacket.NewPacket(buf[:ci.CaptureLength], layers.LayerTypeEthernet, gopacket.DecodeOptions{Lazy: true, NoCopy: true})
		for _, h := range handlers {
			h.Handle(pkt)
		}
	}
}
