package sniffer

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"net"
	"strings"
	"testing"
)

// buildARPPacket собирает Ethernet-кадр с ARP-пакетом (Ethernet/IPv4) для тестов.
func buildARPPacket(t *testing.T, op uint16, srcMAC, dstMAC net.HardwareAddr, srcIP, dstIP net.IP) []byte {
	t.Helper()
	frame := make([]byte, ethHeaderLen+arpPacketLen)

	// Ethernet-заголовок: dst(6) + src(6) + ethertype(2).
	copy(frame[0:6], dstMAC)
	copy(frame[6:12], srcMAC)
	binary.BigEndian.PutUint16(frame[12:14], etherTypeARP)

	// ARP-пакет.
	p := frame[ethHeaderLen:]
	binary.BigEndian.PutUint16(p[0:2], 1)      // HType: Ethernet
	binary.BigEndian.PutUint16(p[2:4], 0x0800) // PType: IPv4
	p[4] = 6                                   // HLEN: 6 байт
	p[5] = 4                                   // PLEN: 4 байта
	binary.BigEndian.PutUint16(p[6:8], op)     // Operation
	copy(p[8:14], srcMAC)                      // SHA
	copy(p[14:18], srcIP.To4())                // SPA
	copy(p[18:24], dstMAC)                     // THA
	copy(p[24:28], dstIP.To4())                // TPA
	return frame
}

// buildIPv4Frame собирает обычный IPv4-кадр (не ARP) для тестов.
func buildIPv4Frame() []byte {
	frame := make([]byte, ethHeaderLen+20)
	copy(frame[0:6], net.HardwareAddr{0x00, 0xaa, 0xbb, 0xcc, 0xdd, 0xee})
	copy(frame[6:12], net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55})
	binary.BigEndian.PutUint16(frame[12:14], 0x0800) // EtherType: IPv4
	return frame
}

// newTestLogger возвращает логгер, пишущий в буфер.
func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, nil))
}

func TestARPHandlerHandleRequest(t *testing.T) {
	var buf bytes.Buffer
	h := NewARPHandler(newTestLogger(&buf))

	frame := buildARPPacket(t,
		arpRequest,
		net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		net.IP{192, 168, 1, 10},
		net.IP{192, 168, 1, 1},
	)
	h.Handle(frame)

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

	frame := buildARPPacket(t,
		arpReply,
		net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		net.HardwareAddr{0x00, 0xaa, 0xbb, 0xcc, 0xdd, 0xee},
		net.IP{192, 168, 1, 1},
		net.IP{192, 168, 1, 10},
	)
	h.Handle(frame)

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
	h.Handle(buildIPv4Frame())

	if out := buf.String(); out != "" {
		t.Errorf("не-ARP пакет не должен логироваться, получено:\n%s", out)
	}
}

func TestParseARPRejectsShortFrame(t *testing.T) {
	// Кадр короче Ethernet-заголовка + ARP-пакета — должен быть отклонён.
	if _, ok := parseARP([]byte{0x00, 0x11, 0x22}); ok {
		t.Error("короткий кадр не должен распознаваться как ARP")
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
