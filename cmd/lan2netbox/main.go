// Альтернативная точка входа: go build ./cmd/lan2netbox.
package main

import (
	"github.com/kirsrus/lan2netbox/internal/cli"
)

func main() {
	cli.Execute()
}
