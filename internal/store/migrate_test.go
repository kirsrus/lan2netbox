package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// openTestDB открывает временную БД для тестов (миграции применяются
// автоматически внутри Open).
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// tableExists проверяет наличие таблицы в схеме SQLite.
func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		name,
	).Scan(&n)
	if err != nil {
		t.Fatalf("проверка таблицы %q: %v", name, err)
	}
	return n > 0
}

// insertMac вставляет тестовую запись MAC-адреса.
func insertMac(t *testing.T, db *sql.DB, mac string) string {
	t.Helper()
	id := NewID()
	if _, err := db.Exec(
		`INSERT INTO mac (id, mac, first_seen, last_seen, seen_count) VALUES (?, ?, ?, ?, ?)`,
		id, mac, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", 1,
	); err != nil {
		t.Fatalf("вставка mac %q: %v", mac, err)
	}
	return id
}

func TestMigrateCreatesTables(t *testing.T) {
	db := openTestDB(t)

	for _, name := range []string{"schema_migrations", "mac", "mac_ip"} {
		if !tableExists(t, db, name) {
			t.Errorf("таблица %q не создана", name)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := openTestDB(t)

	// Повторный вызов не должен приводить к ошибке и дублированию версий.
	if err := Migrate(db); err != nil {
		t.Fatalf("повторный Migrate(): %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("чтение schema_migrations: %v", err)
	}
	if count != len(migrations) {
		t.Errorf("применено миграций %d, ожидалось %d", count, len(migrations))
	}
}

func TestMigrateRecordsVersion(t *testing.T) {
	db := openTestDB(t)

	var version int
	var name string
	err := db.QueryRow(
		`SELECT version, name FROM schema_migrations ORDER BY version LIMIT 1`,
	).Scan(&version, &name)
	if err != nil {
		t.Fatalf("чтение версии миграции: %v", err)
	}
	if version != 1 {
		t.Errorf("версия миграции = %d, ожидалось 1", version)
	}
	if name == "" {
		t.Error("имя миграции не записано")
	}
}

func TestMacUniqueConstraint(t *testing.T) {
	db := openTestDB(t)

	insertMac(t, db, "00:11:22:33:44:55")

	// Второй раз тот же MAC вставить нельзя (UNIQUE).
	if _, err := db.Exec(
		`INSERT INTO mac (id, mac, first_seen, last_seen, seen_count) VALUES (?, ?, ?, ?, ?)`,
		NewID(), "00:11:22:33:44:55", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", 1,
	); err == nil {
		t.Error("вставка дубликата MAC не должна проходить (UNIQUE)")
	}
}

func TestMacIPForeignKeyCascade(t *testing.T) {
	db := openTestDB(t)

	macID := insertMac(t, db, "00:11:22:33:44:55")

	// Одному MAC можно сопоставить несколько IP.
	for _, ip := range []string{"192.168.1.10", "192.168.1.11"} {
		if _, err := db.Exec(
			`INSERT INTO mac_ip (id, mac_id, ip, first_seen, last_seen, seen_count) VALUES (?, ?, ?, ?, ?, ?)`,
			NewID(), macID, ip, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", 1,
		); err != nil {
			t.Fatalf("вставка mac_ip %q: %v", ip, err)
		}
	}

	// Дубликат пары (mac_id, ip) запрещён (UNIQUE).
	if _, err := db.Exec(
		`INSERT INTO mac_ip (id, mac_id, ip, first_seen, last_seen, seen_count) VALUES (?, ?, ?, ?, ?, ?)`,
		NewID(), macID, "192.168.1.10", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", 1,
	); err == nil {
		t.Error("вставка дубликата пары (mac_id, ip) не должна проходить (UNIQUE)")
	}

	// Удаляем MAC — связанные записи mac_ip должны удалиться каскадно.
	if _, err := db.Exec(`DELETE FROM mac WHERE id = ?`, macID); err != nil {
		t.Fatalf("удаление mac: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mac_ip WHERE mac_id = ?`, macID).Scan(&n); err != nil {
		t.Fatalf("чтение mac_ip: %v", err)
	}
	if n != 0 {
		t.Errorf("после удаления mac осталось %d записей mac_ip, ожидалось 0", n)
	}
}

func TestNewID(t *testing.T) {
	// ID должен быть непустым, коротким и уникальным.
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := NewID()
		if id == "" {
			t.Fatal("NewID() вернул пустую строку")
		}
		if len(id) > 20 {
			t.Errorf("NewID() = %q длиной %d, ожидалось не более 20 символов", id, len(id))
		}
		if seen[id] {
			t.Fatalf("NewID() вернул дубликат %q", id)
		}
		seen[id] = true
	}
}
