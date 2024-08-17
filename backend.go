package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
)

type Backend struct {
	Address string
	Weight  int64

	filePath string
}

func NewBackend(address string, path string) Backend {
	return Backend{
		Address: address,
		Weight:  0,

		filePath: path,
	}
}

func (b *Backend) Update() error {
	buffer, err := os.ReadFile(b.filePath)
	if err != nil {
		return errors.New(
			fmt.Sprintf("error opening backend file: %s\n", err.Error()),
		)
	}

	var weight int64
	fmt.Fscanf(bytes.NewReader(buffer), "%d", &weight)

	b.Weight = weight

	return nil
}
