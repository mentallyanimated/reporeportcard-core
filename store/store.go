package store

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound = errors.New("not found")
)

type ErrInternalPutFailed struct {
	key   string
	value []byte
	err   error
}

func (e ErrInternalPutFailed) Error() string {
	return fmt.Sprintf("put for underlying implementation failed: %v", e.err)
}

type Store interface {
	Get(key string) ([]byte, error)
	Put(key string, value []byte) error
}
