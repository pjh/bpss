package main

import (
	"fmt"
)

func main() {
	petServer, err := pets.NewServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
}
