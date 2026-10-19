package sniffer

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/kirsrus/lan2netbox/internal/store"
)

// openTestDB открывает временную БД SQLite для тестов кэша
// (миграции схемы применяются автоматически внутри store.Open).
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("store.Open(): %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// countRows возвращает количество строк в таблице.
func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("чтение COUNT(*) из %s: %v", table, err)
	}
	return n
}

// TestCacheLoadEmpty проверяет загрузку кэша из пустой БД.
func TestCacheLoadEmpty(t *testing.T) {
	db := openTestDB(t)
	cache := newDeviceCache(db)

	if err := cache.Load(); err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if len(cache.macs) != 0 {
		t.Errorf("macs = %d, ожидалось 0", len(cache.macs))
	}
	if len(cache.macIPs) != 0 {
		t.Errorf("macIPs = %d, ожидалось 0", len(cache.macIPs))
	}
}

// TestCacheUpsertAndFlush проверяет добавление новых записей в кэш,
// их сохранение в БД и возврат сохранённых записей MAC.
func TestCacheUpsertAndFlush(t *testing.T) {
	db := openTestDB(t)
	cache := newDeviceCache(db)
	if err := cache.Load(); err != nil {
		t.Fatalf("Load(): %v", err)
	}

	now := "2026-01-01T00:00:00Z"
	macID := cache.UpsertMAC("00:11:22:33:44:55", now)
	if macID == "" {
		t.Fatal("UpsertMAC вернул пустой id")
	}
	cache.UpsertMACIP(macID, "192.168.1.10", now)

	// Повторное обнаружение того же MAC/IP увеличивает счётчики.
	cache.UpsertMAC("00:11:22:33:44:55", "2026-01-01T00:00:01Z")
	cache.UpsertMACIP(macID, "192.168.1.10", "2026-01-01T00:00:01Z")

	saved, err := cache.Flush()
	if err != nil {
		t.Fatalf("Flush(): %v", err)
	}
	if len(saved) != 1 {
		t.Fatalf("Flush вернул %d записей, ожидалось 1", len(saved))
	}
	if saved[0].id != macID || saved[0].mac != "00:11:22:33:44:55" {
		t.Errorf("сохранённая запись = %+v", saved[0])
	}
	if saved[0].seenCount != 2 {
		t.Errorf("seen_count = %d, ожидалось 2", saved[0].seenCount)
	}

	// Данные должны быть в БД.
	if n := countRows(t, db, "mac"); n != 1 {
		t.Errorf("в таблице mac %d строк, ожидалось 1", n)
	}
	if n := countRows(t, db, "mac_ip"); n != 1 {
		t.Errorf("в таблице mac_ip %d строк, ожидалось 1", n)
	}

	// Повторный Flush без изменений не должен ничего возвращать.
	saved, err = cache.Flush()
	if err != nil {
		t.Fatalf("повторный Flush(): %v", err)
	}
	if len(saved) != 0 {
		t.Errorf("повторный Flush вернул %d записей, ожидалось 0", len(saved))
	}
}

// TestCacheLoadFromDB проверяет загрузку существующих записей из БД
// и обновление их при повторном обнаружении (без дублирования).
func TestCacheLoadFromDB(t *testing.T) {
	db := openTestDB(t)

	// Предварительно заполняем БД напрямую.
	macID := store.NewID()
	if _, err := db.Exec(
		`INSERT INTO mac (id, mac, first_seen, last_seen, seen_count) VALUES (?, ?, ?, ?, ?)`,
		macID, "00:aa:bb:cc:dd:ee", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", 1,
	); err != nil {
		t.Fatalf("вставка mac: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO mac_ip (id, mac_id, ip, first_seen, last_seen, seen_count) VALUES (?, ?, ?, ?, ?, ?)`,
		store.NewID(), macID, "10.0.0.5", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", 1,
	); err != nil {
		t.Fatalf("вставка mac_ip: %v", err)
	}

	cache := newDeviceCache(db)
	if err := cache.Load(); err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if len(cache.macs) != 1 || len(cache.macIPs) != 1 {
		t.Fatalf("кэш: macs=%d macIPs=%d, ожидалось 1 и 1", len(cache.macs), len(cache.macIPs))
	}

	// Повторное обнаружение существующего MAC — тот же id, счётчик растёт.
	gotID := cache.UpsertMAC("00:aa:bb:cc:dd:ee", "2026-01-01T00:01:00Z")
	if gotID != macID {
		t.Errorf("UpsertMAC вернул id %q, ожидалось %q", gotID, macID)
	}
	cache.UpsertMACIP(macID, "10.0.0.5", "2026-01-01T00:01:00Z")

	saved, err := cache.Flush()
	if err != nil {
		t.Fatalf("Flush(): %v", err)
	}
	if len(saved) != 1 {
		t.Fatalf("Flush вернул %d записей, ожидалось 1", len(saved))
	}
	if saved[0].seenCount != 2 {
		t.Errorf("seen_count = %d, ожидалось 2", saved[0].seenCount)
	}

	// Дубликатов в БД быть не должно.
	if n := countRows(t, db, "mac"); n != 1 {
		t.Errorf("в таблице mac %d строк, ожидалось 1", n)
	}
	if n := countRows(t, db, "mac_ip"); n != 1 {
		t.Errorf("в таблице mac_ip %d строк, ожидалось 1", n)
	}
}

// TestCacheFlushNoDirty проверяет, что Flush без изменений не
// обращается к БД и возвращает пустой список.
func TestCacheFlushNoDirty(t *testing.T) {
	db := openTestDB(t)
	cache := newDeviceCache(db)
	if err := cache.Load(); err != nil {
		t.Fatalf("Load(): %v", err)
	}

	saved, err := cache.Flush()
	if err != nil {
		t.Fatalf("Flush(): %v", err)
	}
	if len(saved) != 0 {
		t.Errorf("Flush вернул %d записей, ожидалось 0", len(saved))
	}
}
