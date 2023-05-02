package main

import (
	"fmt"

	"github.com/pjh/bpss/pets"
)

func main() {
	petServer, err := pets.NewServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
}
