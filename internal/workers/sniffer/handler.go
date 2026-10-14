// Package sniffer — горутина обнаружения сетевых устройств.
//
// Sniffer — издатель событий шины: публикует device.discovered,
// device.updated и device.deleted. Захват пакетов выполняется через
// github.com/google/gopacket/afpacket (AF_PACKET, Linux, без cgo).
// Обработчики пакетов (ARP, DHCP, mDNS) регистрируются в компоненте
// и вызываются для каждого захваченного кадра.
package sniffer

import "github.com/google/gopacket"

// PacketHandler — обработчик захваченных пакетов.
//
// Каждый протокол (ARP, DHCP, mDNS и т.д.) реализует свой обработчик
// и регистрируется в Sniffer. Это позволяет расширять сбор данных
// без изменения цикла захвата: достаточно добавить новый обработчик
// и зарегистрировать его (см. Sniffer.RegisterHandler).
type PacketHandler interface {
	// Name возвращает имя обработчика (протокола) для логов.
	Name() string

	// Handle вызывается для каждого захваченного пакета.
	// Реализация должна быть быстрой и не блокирующей: цикл захвата
	// синхронно вызывает все зарегистрированные обработчики.
	Handle(pkt gopacket.Packet)
}
