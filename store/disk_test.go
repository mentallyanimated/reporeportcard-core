package store

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SimpleReadAndWrite(t *testing.T) {
	assert := assert.New(t)
	owner := "foo"
	repo := "bar"
	expected := []byte("bar")
	diskStore := NewDisk(owner, repo)
	diskStore.Put("foo", expected)

	actual, err := diskStore.Get("foo")
	assert.Nil(err)
	assert.Equal(expected, actual)
}

func Test_NotFound(t *testing.T) {
	assert := assert.New(t)
	owner := "foo"
	repo := "bar"
	diskStore := NewDisk(owner, repo)

	actual, err := diskStore.Get("foo")
	assert.Nil(actual)
	assert.True(errors.Is(err, ErrNotFound))
}
