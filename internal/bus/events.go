package bus

import "time"

// Контракты событий шины. Payload публикуется в топик в виде JSON.
//
// Внимание: на этапе появления models/ эти структуры могут быть
// заменены/расширены ссылками на модели (models.Device и т.п.) —
// здесь зафиксированы минимальные поля, необходимые подписчикам.

// DeviceDiscoveredEvent — новое устройство обнаружено в сети
// (топик TopicDeviceDiscovered, издатель: sniffer).
type DeviceDiscoveredEvent struct {
	// MAC — нормализованный MAC-адрес устройства.
	MAC string `json:"mac"`
	// IP — последний известный IP-адрес.
	IP string `json:"ip,omitempty"`
	// Source — источник обнаружения: arp | dhcp | mdns | scan.
	Source string `json:"source,omitempty"`
	// Timestamp — время обнаружения.
	Timestamp time.Time `json:"timestamp"`
}

// DeviceUpdatedEvent — запись устройства обновлена
// (топик TopicDeviceUpdated, издатели: sniffer, probe).
type DeviceUpdatedEvent struct {
	// MAC — нормализованный MAC-адрес устройства.
	MAC string `json:"mac"`
	// IP — актуальный IP-адрес.
	IP string `json:"ip,omitempty"`
	// Hostname — имя устройства (sysName, reverse DNS и т.п.).
	Hostname string `json:"hostname,omitempty"`
	// Vendor — вендор по MAC (OUI).
	Vendor string `json:"vendor,omitempty"`
	// LastSeen — время последней активности.
	LastSeen time.Time `json:"last_seen"`
}

// DeviceDeletedEvent — устройство пропало из сети
// (топик TopicDeviceDeleted, издатель: sniffer).
type DeviceDeletedEvent struct {
	// MAC — нормализованный MAC-адрес устройства.
	MAC string `json:"mac"`
	// Timestamp — время удаления.
	Timestamp time.Time `json:"timestamp"`
}

// Link — связь устройство ↔ коммутатор.
type Link struct {
	// LocalMAC — MAC устройства, с которым связана запись.
	LocalMAC string `json:"local_mac"`
	// RemoteMAC — MAC коммутатора (пусто для связей без соседа).
	RemoteMAC string `json:"remote_mac,omitempty"`
	// RemotePort — порт на коммутаторе.
	RemotePort string `json:"remote_port,omitempty"`
	// Method — метод обнаружения: snmp_fdb | snmp_lldp | http.
	Method string `json:"method"`
}

// LinkChangedEvent — изменились связи устройство-коммутатор
// (топик TopicLinkChanged, издатель: probe).
type LinkChangedEvent struct {
	// MAC — устройство, связи которого изменились.
	MAC string `json:"mac"`
	// Links — актуальный набор связей устройства.
	Links []Link `json:"links"`
	// Timestamp — время изменения.
	Timestamp time.Time `json:"timestamp"`
}

// SyncRequestEvent — принудительная синхронизация с NetBox
// (топик TopicSyncRequest, издатель: web).
type SyncRequestEvent struct {
	// Reason — причина запуска (например, действие оператора).
	Reason string `json:"reason,omitempty"`
	// Requested — время запроса.
	Requested time.Time `json:"requested"`
}
