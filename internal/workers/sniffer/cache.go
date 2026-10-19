// Кэш обнаруженных устройств (MAC и связи MAC↔IP).
//
// При старте sniffer кэш загружается из локальной БД SQLite (таблицы
// mac и mac_ip). Обработчики пакетов (ARP и др.) обновляют кэш при
// обнаружении новых MAC-адресов и IP: изменённые записи помечаются
// флагом dirty («не синхронизирована с БД»). Периодически (см.
// SnifferConfig.FlushInterval) изменённые записи сохраняются в БД
// методом Flush, который возвращает сохранённые записи MAC — их id
// публикуются в шину событий (топик device.saved).
//
// Кэш потокобезопасен: обновляется из цикла захвата (обработчики
// пакетов) и из горутины синхронизации (Flush) параллельно, поэтому
// доступ к нему защищён мьютексом.
package sniffer

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/kirsrus/lan2netbox/internal/store"
)

// macRecord — запись кэша таблицы mac.
type macRecord struct {
	// id — идентификатор записи (xid, см. store.NewID).
	id string
	// mac — канонический MAC-адрес "aa:bb:cc:dd:ee:ff" (нижний регистр).
	mac string
	// firstSeen — время первого обнаружения (RFC3339, UTC).
	firstSeen string
	// lastSeen — время последнего обнаружения (RFC3339, UTC).
	lastSeen string
	// seenCount — количество обнаружений.
	seenCount int
	// dirty — запись изменена и ещё не синхронизирована с БД.
	dirty bool
}

// macIPRecord — запись кэша таблицы mac_ip (связь MAC↔IP).
type macIPRecord struct {
	// id — идентификатор записи (xid).
	id string
	// macID — ссылка на запись mac (macRecord.id).
	macID string
	// ip — канонический IP-адрес "192.168.1.10".
	ip string
	// firstSeen — время первого обнаружения (RFC3339, UTC).
	firstSeen string
	// lastSeen — время последнего обнаружения (RFC3339, UTC).
	lastSeen string
	// seenCount — количество обнаружений.
	seenCount int
	// dirty — запись изменена и ещё не синхронизирована с БД.
	dirty bool
}

// DeviceCache — локальный кэш обнаруженных устройств.
//
// Ключи: macs — канонический MAC-адрес; macIPs — "macID|ip".
type DeviceCache struct {
	// mu — защищает доступ к кэшу из цикла захвата и горутины
	// синхронизации.
	mu sync.Mutex
	// db — локальная база данных SQLite.
	db *sql.DB
	// macs — записи MAC-адресов.
	macs map[string]macRecord
	// macIPs — связи MAC↔IP.
	macIPs map[string]macIPRecord
}

// newDeviceCache создаёт пустой кэш устройств.
func newDeviceCache(db *sql.DB) *DeviceCache {
	return &DeviceCache{
		db:     db,
		macs:   make(map[string]macRecord),
		macIPs: make(map[string]macIPRecord),
	}
}

// Load загружает содержимое таблиц mac и mac_ip из БД в кэш.
// Вызывается при старте sniffer; повторный вызов перезагружает кэш.
func (c *DeviceCache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.loadMACs(); err != nil {
		return err
	}
	return c.loadMACIPs()
}

// loadMACs читает таблицу mac в кэш.
func (c *DeviceCache) loadMACs() error {
	rows, err := c.db.Query(`SELECT id, mac, first_seen, last_seen, seen_count FROM mac`)
	if err != nil {
		return fmt.Errorf("sniffer: чтение таблицы mac: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, mac, firstSeen, lastSeen string
		var seenCount int
		if err := rows.Scan(&id, &mac, &firstSeen, &lastSeen, &seenCount); err != nil {
			return fmt.Errorf("sniffer: чтение записи mac: %w", err)
		}
		c.macs[mac] = macRecord{
			id:        id,
			mac:       mac,
			firstSeen: firstSeen,
			lastSeen:  lastSeen,
			seenCount: seenCount,
			dirty:     false,
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sniffer: чтение таблицы mac: %w", err)
	}
	return nil
}

// loadMACIPs читает таблицу mac_ip в кэш.
func (c *DeviceCache) loadMACIPs() error {
	rows, err := c.db.Query(`SELECT id, mac_id, ip, first_seen, last_seen, seen_count FROM mac_ip`)
	if err != nil {
		return fmt.Errorf("sniffer: чтение таблицы mac_ip: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, macID, ip, firstSeen, lastSeen string
		var seenCount int
		if err := rows.Scan(&id, &macID, &ip, &firstSeen, &lastSeen, &seenCount); err != nil {
			return fmt.Errorf("sniffer: чтение записи mac_ip: %w", err)
		}
		c.macIPs[macID+"|"+ip] = macIPRecord{
			id:        id,
			macID:     macID,
			ip:        ip,
			firstSeen: firstSeen,
			lastSeen:  lastSeen,
			seenCount: seenCount,
			dirty:     false,
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sniffer: чтение таблицы mac_ip: %w", err)
	}
	return nil
}

// UpsertMAC обновляет или создаёт запись MAC-адреса в кэше.
//
// now — время обнаружения в формате RFC3339 (UTC). Возвращает id
// записи (существующей или новой). Запись помечается как не
// синхронизированная с БД (dirty=true).
func (c *DeviceCache) UpsertMAC(mac string, now string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rec, ok := c.macs[mac]; ok {
		rec.lastSeen = now
		rec.seenCount += 1
		rec.dirty = true
		c.macs[mac] = rec
		return rec.id
	}
	rec := macRecord{
		id:        store.NewID(),
		mac:       mac,
		firstSeen: now,
		lastSeen:  now,
		seenCount: 1,
		dirty:     true,
	}
	c.macs[mac] = rec
	return rec.id
}

// UpsertMACIP обновляет или создаёт связь MAC↔IP в кэше.
//
// macID — id записи MAC (см. UpsertMAC), ip — канонический IP-адрес,
// now — время обнаружения (RFC3339, UTC). Запись помечается dirty.
func (c *DeviceCache) UpsertMACIP(macID string, ip string, now string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := macID + "|" + ip
	if rec, ok := c.macIPs[key]; ok {
		rec.lastSeen = now
		rec.seenCount += 1
		rec.dirty = true
		c.macIPs[key] = rec
		return rec.id
	}
	rec := macIPRecord{
		id:        store.NewID(),
		macID:     macID,
		ip:        ip,
		firstSeen: now,
		lastSeen:  now,
		seenCount: 1,
		dirty:     true,
	}
	c.macIPs[key] = rec
	return rec.id
}

// Flush сохраняет изменённые (dirty) записи кэша в БД в одной
// транзакции и возвращает сохранённые записи MAC — их id публикуются
// в шину событий (device.saved). Если изменений нет, возвращается
// пустой список без обращения к БД.
func (c *DeviceCache) Flush() ([]macRecord, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Собираем ключи изменённых записей до начала транзакции, чтобы
	// не модифицировать map во время итерации.
	var dirtyMACs []string
	for mac, rec := range c.macs {
		if rec.dirty {
			dirtyMACs = append(dirtyMACs, mac)
		}
	}
	var dirtyMACIPs []string
	for key, rec := range c.macIPs {
		if rec.dirty {
			dirtyMACIPs = append(dirtyMACIPs, key)
		}
	}
	if len(dirtyMACs) == 0 && len(dirtyMACIPs) == 0 {
		return nil, nil
	}

	tx, err := c.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("sniffer: начало транзакции сохранения кэша: %w", err)
	}
	// Rollback после успешного Commit — no-op, безопасен.
	defer tx.Rollback()

	saved := make([]macRecord, 0, len(dirtyMACs))

	// Сохраняем изменённые записи MAC. UPSERT покрывает и новые записи
	// (INSERT), и обновление существующих (last_seen, seen_count).
	for _, mac := range dirtyMACs {
		rec, ok := c.macs[mac]
		if !ok {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO mac (id, mac, first_seen, last_seen, seen_count) VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET last_seen = excluded.last_seen, seen_count = excluded.seen_count`,
			rec.id, rec.mac, rec.firstSeen, rec.lastSeen, rec.seenCount,
		); err != nil {
			return nil, fmt.Errorf("sniffer: сохранение записи mac %q: %w", rec.mac, err)
		}
		rec.dirty = false
		c.macs[mac] = rec
		saved = append(saved, rec)
	}

	// Сохраняем изменённые связи MAC↔IP.
	for _, key := range dirtyMACIPs {
		rec, ok := c.macIPs[key]
		if !ok {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO mac_ip (id, mac_id, ip, first_seen, last_seen, seen_count) VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET last_seen = excluded.last_seen, seen_count = excluded.seen_count`,
			rec.id, rec.macID, rec.ip, rec.firstSeen, rec.lastSeen, rec.seenCount,
		); err != nil {
			return nil, fmt.Errorf("sniffer: сохранение связи mac_ip %q: %w", rec.ip, err)
		}
		rec.dirty = false
		c.macIPs[key] = rec
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("sniffer: фиксация транзакции сохранения кэша: %w", err)
	}
	return saved, nil
}
