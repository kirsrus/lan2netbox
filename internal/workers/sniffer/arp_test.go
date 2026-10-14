package sniffer

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// buildARPPacket собирает Ethernet-кадр с ARP-пакетом для тестов.
func buildARPPacket(t *testing.T, op uint16, srcMAC, dstMAC net.HardwareAddr, srcIP, dstIP net.IP) gopacket.Packet {
	t.Helper()
	eth := &layers.Ethernet{
		SrcMAC:       srcMAC,
		DstMAC:       dstMAC,
		EthernetType: layers.EthernetTypeARP,
	}
	arp := &layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         op,
		SourceHwAddress:   srcMAC,
		SourceProtAddress: srcIP,
		DstHwAddress:      dstMAC,
		DstProtAddress:    dstIP,
	}
	buf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{}, eth, arp); err != nil {
		t.Fatalf("сборка ARP-кадра: %v", err)
	}
	return gopacket.NewPacket(buf.Bytes(), layers.LayerTypeEthernet, gopacket.Default)
}

// newTestLogger возвращает логгер, пишущий в буфер.
func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, nil))
}

func TestARPHandlerHandleRequest(t *testing.T) {
	var buf bytes.Buffer
	h := NewARPHandler(newTestLogger(&buf))

	pkt := buildARPPacket(t,
		layers.ARPRequest,
		net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		net.IP{192, 168, 1, 10},
		net.IP{192, 168, 1, 1},
	)
	h.Handle(pkt)

	out := buf.String()
	for _, want := range []string{"ARP-запрос", "00:11:22:33:44:55", "192.168.1.10", "192.168.1.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("лог не содержит %q:\n%s", want, out)
		}
	}
}

func TestARPHandlerHandleReply(t *testing.T) {
	var buf bytes.Buffer
	h := NewARPHandler(newTestLogger(&buf))

	pkt := buildARPPacket(t,
		layers.ARPReply,
		net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		net.HardwareAddr{0x00, 0xaa, 0xbb, 0xcc, 0xdd, 0xee},
		net.IP{192, 168, 1, 1},
		net.IP{192, 168, 1, 10},
	)
	h.Handle(pkt)

	out := buf.String()
	for _, want := range []string{"ARP-ответ", "00:11:22:33:44:55", "192.168.1.1", "00:aa:bb:cc:dd:ee"} {
		if !strings.Contains(out, want) {
			t.Errorf("лог не содержит %q:\n%s", want, out)
		}
	}
}

func TestARPHandlerIgnoresNonARP(t *testing.T) {
	var buf bytes.Buffer
	h := NewARPHandler(newTestLogger(&buf))

	// Обычный IPv4-кадр (не ARP) — обработчик должен его проигнорировать.
	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		DstMAC:       net.HardwareAddr{0x00, 0xaa, 0xbb, 0xcc, 0xdd, 0xee},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolICMPv4,
		SrcIP:    net.IP{192, 168, 1, 10},
		DstIP:    net.IP{192, 168, 1, 1},
	}
	icmp := &layers.ICMPv4{TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0)}
	serBuf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(serBuf, gopacket.SerializeOptions{}, eth, ip, icmp); err != nil {
		t.Fatalf("сборка IPv4-кадра: %v", err)
	}
	pkt := gopacket.NewPacket(serBuf.Bytes(), layers.LayerTypeEthernet, gopacket.Default)

	h.Handle(pkt)

	if out := buf.String(); out != "" {
		t.Errorf("не-ARP пакет не должен логироваться, получено:\n%s", out)
	}
}

func TestResolveInterface(t *testing.T) {
	// Заданный интерфейс возвращается как есть.
	name, err := resolveInterface("eth0")
	if err != nil {
		t.Fatalf("resolveInterface: %v", err)
	}
	if name != "eth0" {
		t.Errorf("resolveInterface(\"eth0\") = %q, ожидалось \"eth0\"", name)
	}

	// Пустой интерфейс — автоопределение (зависит от окружения, но не должно падать).
	if _, err := resolveInterface(""); err != nil {
		t.Logf("автоопределение интерфейса: %v (допустимо в изолированном окружении)", err)
	}
}
