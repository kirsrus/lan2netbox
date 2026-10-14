package sniffer

import (
	"log/slog"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// ARPHandler — обработчик ARP-пакетов.
//
// Выводит в лог (консоль) MAC-адреса, привязанные к ним IP-адреса
// и запрашиваемые IP-адреса из ARP-запросов и ARP-ответов.
// В дальнейшем на основе этих данных будут публиковаться события
// device.discovered / device.updated.
type ARPHandler struct {
	logger *slog.Logger
}

// NewARPHandler создаёт обработчик ARP-пакетов.
func NewARPHandler(logger *slog.Logger) *ARPHandler {
	return &ARPHandler{logger: logger}
}

// Name возвращает имя обработчика.
func (h *ARPHandler) Name() string { return "arp" }

// Handle обрабатывает пакет: извлекает ARP-слой и логирует его содержимое.
func (h *ARPHandler) Handle(pkt gopacket.Packet) {
	arpLayer := pkt.Layer(layers.LayerTypeARP)
	if arpLayer == nil {
		return
	}
	arp, ok := arpLayer.(*layers.ARP)
	if !ok {
		return
	}

	senderMAC := net.HardwareAddr(arp.SourceHwAddress)
	senderIP := net.IP(arp.SourceProtAddress)
	targetIP := net.IP(arp.DstProtAddress)

	switch arp.Operation {
	case layers.ARPRequest:
		// ARP-запрос: «кто владеет targetIP?» — отправитель senderMAC/senderIP.
		h.logger.Info("ARP-запрос",
			"sender_mac", senderMAC.String(),
			"sender_ip", senderIP.String(),
			"target_ip", targetIP.String(),
		)
	case layers.ARPReply:
		// ARP-ответ: senderMAC объявляет, что владеет senderIP.
		targetMAC := net.HardwareAddr(arp.DstHwAddress)
		h.logger.Info("ARP-ответ",
			"sender_mac", senderMAC.String(),
			"sender_ip", senderIP.String(),
			"target_mac", targetMAC.String(),
			"target_ip", targetIP.String(),
		)
	default:
		h.logger.Debug("ARP-пакет с неизвестной операцией",
			"operation", arp.Operation,
			"sender_mac", senderMAC.String(),
			"sender_ip", senderIP.String(),
		)
	}
}
