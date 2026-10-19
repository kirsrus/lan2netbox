package store

// migrations — все миграции схемы БД в порядке применения.
//
// Новые миграции добавляются в конец списка с версией на единицу больше
// предыдущей. Уже применённые миграции не изменяются: для изменения
// схемы создаётся новая миграция (ALTER TABLE и т.п.).
var migrations = []migration{
	{
		version: 1,
		name:    "create mac and mac_ip tables",
		statements: []string{
			// Таблица MAC-адресов, обнаруженных sniffer.
			//
			// id — короткий глобально уникальный идентификатор (xid,
			// 20 символов base32), сортируемый по времени создания.
			// mac — канонический вид "aa:bb:cc:dd:ee:ff" (нижний регистр).
			// first_seen/last_seen — время первого/последнего обнаружения
			// в формате RFC3339 (UTC).
			// seen_count — количество обнаружений.
			// manual_description — ручное описание устройства.
			// vendor — автоматическое описание производителя (OUI),
			// заполняется позже из таблицы производителей.
			`CREATE TABLE IF NOT EXISTS mac (
    id                 TEXT PRIMARY KEY,
    mac                TEXT NOT NULL UNIQUE,
    first_seen         TEXT NOT NULL,
    last_seen          TEXT NOT NULL,
    seen_count         INTEGER NOT NULL DEFAULT 1 CHECK (seen_count >= 1),
    manual_description TEXT NOT NULL DEFAULT '',
    vendor             TEXT NOT NULL DEFAULT ''
)`,
			// Индекс для поиска активных устройств по времени последнего обнаружения.
			`CREATE INDEX IF NOT EXISTS idx_mac_last_seen ON mac(last_seen)`,

			// Таблица связей MAC-адрес ↔ IP-адрес: одному MAC может
			// принадлежать несколько IP (и наоборот — один IP может
			// встречаться у разных MAC, например при смене устройства).
			//
			// mac_id — ссылка на mac.id; при удалении MAC связанные
			// записи удаляются каскадно (ON DELETE CASCADE).
			// ip — канонический вид "192.168.1.10".
			// first_seen/last_seen/seen_count — метрики обнаружения пары.
			`CREATE TABLE IF NOT EXISTS mac_ip (
    id         TEXT PRIMARY KEY,
    mac_id     TEXT NOT NULL REFERENCES mac(id) ON DELETE CASCADE,
    ip         TEXT NOT NULL,
    first_seen TEXT NOT NULL,
    last_seen  TEXT NOT NULL,
    seen_count INTEGER NOT NULL DEFAULT 1 CHECK (seen_count >= 1),
    UNIQUE (mac_id, ip)
)`,
			// Индексы для поиска по MAC и по IP.
			`CREATE INDEX IF NOT EXISTS idx_mac_ip_mac_id ON mac_ip(mac_id)`,
			`CREATE INDEX IF NOT EXISTS idx_mac_ip_ip ON mac_ip(ip)`,
		},
	},
}
