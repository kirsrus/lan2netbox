package sniffer

import (
	"encoding/binary"
	"log/slog"
	"net"
)

// Константы протокола ARP (RFC 826).
const (
	// etherTypeARP — EtherType кадра ARP.
	etherTypeARP = 0x0806
	// arpRequest — операция «ARP-запрос» (кто владеет targetIP?).
	arpRequest = 1
	// arpReply — операция «ARP-ответ» (senderMAC владеет senderIP).
	arpReply = 2
)

// Размеры заголовков.
const (
	// ethHeaderLen — длина Ethernet-заголовка: dst(6) + src(6) + ethertype(2).
	ethHeaderLen = 14
	// arpPacketLen — длина ARP-пакета для Ethernet/IPv4:
	// htype(2) + ptype(2) + hlen(1) + plen(1) + oper(2) + sha(6) + spa(4) + tha(6) + tpa(4).
	arpPacketLen = 28
)

// arpPacket — разобранный ARP-пакет (Ethernet/IPv4).
type arpPacket struct {
	operation uint16
	senderMAC net.HardwareAddr
	senderIP  net.IP
	targetMAC net.HardwareAddr
	targetIP  net.IP
}

// parseARP извлекает ARP-пакет из Ethernet-кадра.
//
// Возвращает false, если кадр не является ARP-пакетом формата
// Ethernet/IPv4 (как в оригинальной реализации на gopacket).
// Адреса копируются: кадр может быть переиспользован циклом захвата.
func parseARP(frame []byte) (*arpPacket, bool) {
	if len(frame) < ethHeaderLen+arpPacketLen {
		return nil, false
	}
	// EtherType кадра должен быть ARP.
	if binary.BigEndian.Uint16(frame[12:14]) != etherTypeARP {
		return nil, false
	}

	p := frame[ethHeaderLen:]
	// Проверяем формат: аппаратный тип Ethernet (1), протокол IPv4 (0x0800),
	// 6-байтные MAC-адреса и 4-байтные IP-адреса.
	if binary.BigEndian.Uint16(p[0:2]) != 1 ||
		binary.BigEndian.Uint16(p[2:4]) != 0x0800 ||
		p[4] != 6 || p[5] != 4 {
		return nil, false
	}

	return &arpPacket{
		operation: binary.BigEndian.Uint16(p[6:8]),
		senderMAC: net.HardwareAddr(append([]byte(nil), p[8:14]...)),
		senderIP:  net.IP(append([]byte(nil), p[14:18]...)),
		targetMAC: net.HardwareAddr(append([]byte(nil), p[18:24]...)),
		targetIP:  net.IP(append([]byte(nil), p[24:28]...)),
	}, true
}

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

// Handle обрабатывает Ethernet-кадр: извлекает ARP-пакет и логирует его содержимое.
func (h *ARPHandler) Handle(frame []byte) {
	arp, ok := parseARP(frame)
	if !ok {
		return
	}

	switch arp.operation {
	case arpRequest:
		// ARP-запрос: «кто владеет targetIP?» — отправитель senderMAC/senderIP.
		h.logger.Info("ARP-запрос",
			"sender_mac", arp.senderMAC.String(),
			"sender_ip", arp.senderIP.String(),
			"target_ip", arp.targetIP.String(),
		)
	case arpReply:
		// ARP-ответ: senderMAC объявляет, что владеет senderIP.
		h.logger.Info("ARP-ответ",
			"sender_mac", arp.senderMAC.String(),
			"sender_ip", arp.senderIP.String(),
			"target_mac", arp.targetMAC.String(),
			"target_ip", arp.targetIP.String(),
		)
	default:
		h.logger.Debug("ARP-пакет с неизвестной операцией",
			"operation", arp.operation,
			"sender_mac", arp.senderMAC.String(),
			"sender_ip", arp.senderIP.String(),
		)
	}
}
