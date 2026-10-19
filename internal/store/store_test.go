package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.sqlite")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer db.Close()

	// Проверяем, что соединение действительно работает.
	if err := db.Ping(); err != nil {
		t.Fatalf("Ping(): %v", err)
	}

	// Файл БД должен быть создан на диске.
	if !fileExists(path) {
		t.Errorf("файл БД %q не создан", path)
	}
}

func TestOpenCreatesParentDir(t *testing.T) {
	// Каталог для БД создаётся автоматически.
	path := filepath.Join(t.TempDir(), "nested", "dir", "test.sqlite")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer db.Close()

	if !fileExists(path) {
		t.Errorf("файл БД %q не создан", path)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
