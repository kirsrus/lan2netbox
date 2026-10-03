package cli

import "fmt"

// notImplemented — временная заглушка для команд, реализация которых
// запланирована на следующих этапах разработки.
func notImplemented(name string) error {
	fmt.Printf("[stub] команда %q ещё не реализована\n", name)
	return nil
}
