package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
)

type Backend struct {
	Address string

	score int64

	dir   string
	mutex sync.RWMutex
}

func NewBackend(address string, path string) Backend {
	return Backend{
		Address: address,

		score: 1,

		dir:   path,
		mutex: sync.RWMutex{},
	}
}

func (b *Backend) GetScore() int64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.score
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

	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.score != score {
		log.Printf(
			"backend %s score updated (%d -> %d)", b.Address, b.score, score,
		)
	}

	b.score = score

	return nil
}
