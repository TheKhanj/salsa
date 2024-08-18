package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
)

type Backend struct {
	Address string
	Score   int64

	dir string
}

func NewBackend(address string, path string) Backend {
	return Backend{
		Address: address,
		Score:   1,

		dir: path,
	}
}

func (b *Backend) UpdateScore() error {
	buffer, err := os.ReadFile(b.dir + "/score")
	if err != nil {
		return errors.New(
			fmt.Sprintf("error opening backend file: %s\n", err.Error()),
		)
	}

	var score int64
	fmt.Fscanf(bytes.NewReader(buffer), "%d", &score)

	b.Score = score

	return nil
}
