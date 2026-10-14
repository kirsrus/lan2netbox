package sniffer

import (
	"errors"
	"fmt"
	"net"
)

// resolveInterface определяет интерфейс захвата.
//
// Если iface задан в конфигурации — используется он. Если пусто —
// выбирается первый активный интерфейс с IPv4-адресом: для ARP-трафика
// нужен конкретный интерфейс (в отличие от «any», где кадры приходят
// в cooked-формате Linux SLL).
func resolveInterface(iface string) (string, error) {
	if iface != "" {
		return iface, nil
	}
	name, err := defaultInterface()
	if err != nil {
		return "", err
	}
	return name, nil
}

// defaultInterface возвращает имя первого активного интерфейса с IPv4-адресом.
func defaultInterface() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("sniffer: список сетевых интерфейсов: %w", err)
	}
	for _, iface := range ifaces {
		// Пропускаем неактивные и loopback-интерфейсы.
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if ok && ipnet.IP.To4() != nil {
				return iface.Name, nil
			}
		}
	}
	return "", errors.New("sniffer: не найден активный интерфейс с IPv4-адресом")
}
