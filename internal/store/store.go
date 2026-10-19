// Package store — инициализация локальной базы данных SQLite.
//
// Используется порт SQLite https://gitlab.com/cznic/sqlite через обёртку
// https://github.com/glebarez/go-sqlite, реализующую интерфейс database/sql.
// Драйвер регистрируется под именем "sqlite" (см. init пакета glebarez/go-sqlite).
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite" // регистрирует драйвер "sqlite"
)

// Open открывает (создаёт при необходимости) базу данных SQLite
// по указанному пути и возвращает пул соединений database/sql.
//
// Каталог для файла БД создаётся автоматически. Вызов Ping проверяет,
// что база доступна и готова к работе. При подключении применяются
// миграции схемы (см. Migrate): таблицы создаются и обновляются
// автоматически.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: создание каталога %q: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: открытие БД %q: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: проверка соединения с БД %q: %w", path, err)
	}

	// SQLite поддерживает только одного писателя; ограничиваем пул одним
	// соединением, чтобы PRAGMA (foreign_keys) гарантированно применялись
	// ко всем операциям.
	db.SetMaxOpenConns(1)

	// Внешние ключи в SQLite выключены по умолчанию — включаем явно,
	// чтобы работал ON DELETE CASCADE в таблице mac_ip.
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: включение внешних ключей: %w", err)
	}

	// Применяем миграции схемы: таблицы создаются/обновляются при подключении.
	if err := Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: миграция схемы БД %q: %w", path, err)
	}

	return db, nil
}
