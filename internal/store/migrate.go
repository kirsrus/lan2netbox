package store

import (
	"database/sql"
	"fmt"
)

// schemaMigrationsTable — таблица учёта применённых миграций.
//
// Хранит номер версии, имя миграции и время применения. По ней
// определяется, какие миграции уже выполнены, а какие нужно применить.
const schemaMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
)`

// migration — одна миграция схемы базы данных.
type migration struct {
	// version — порядковый номер миграции (строго возрастающий).
	version int
	// name — человекочитаемое имя миграции (для логов и таблицы учёта).
	name string
	// statements — SQL-выражения, выполняемые в одной транзакции.
	//
	// Каждое выражение выполняется отдельным Exec: pure-Go драйвер
	// glebarez/go-sqlite не гарантирует выполнение нескольких
	// выражений, переданных одним вызовом.
	statements []string
}

// Migrate приводит схему БД к актуальному состоянию, применяя все
// неприменённые миграции по порядку возрастания версии.
//
// Функция идемпотентна: повторный вызов не изменяет схему и не
// приводит к ошибке. Вызывается автоматически при подключении
// через Open, а также доступна для явного использования (тесты,
// будущие CLI-команды).
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schemaMigrationsTable); err != nil {
		return fmt.Errorf("store: создание таблицы schema_migrations: %w", err)
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return fmt.Errorf("store: миграция %d (%s): %w", m.version, m.name, err)
		}
	}
	return nil
}

// appliedVersions возвращает множество версий уже применённых миграций.
func appliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("store: чтение применённых миграций: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("store: чтение версии миграции: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: чтение применённых миграций: %w", err)
	}
	return applied, nil
}

// applyMigration выполняет миграцию в одной транзакции: все SQL-выражения
// и запись версии в schema_migrations. При любой ошибке транзакция
// откатывается, и версия не фиксируется — миграция будет повторена
// при следующем вызове Migrate.
func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("начало транзакции: %w", err)
	}
	// Rollback после успешного Commit — no-op, безопасен.
	defer tx.Rollback()

	for _, stmt := range m.statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("выполнение выражения: %w", err)
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`,
		m.version, m.name,
	); err != nil {
		return fmt.Errorf("запись версии миграции: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("фиксация транзакции: %w", err)
	}
	return nil
}
