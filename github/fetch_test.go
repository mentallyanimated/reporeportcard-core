package github

import (
	"context"
	"errors"
	"testing"

	"github.com/mentallyanimated/reporeportcard-core/store"
	"github.com/stretchr/testify/assert"
)

func Test_ListAllPRs(t *testing.T) {
	assert := assert.New(t)
	cache := store.NewDisk()
	client := NewClient(context.TODO(), "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", cache)

	pullRequests, err := client.DownloadMergedPullRequests(context.TODO(), "color", "color")
	if errors.Is(err, store.ErrNotFound) {
		t.Log("metadata not found")
	} else {
		assert.NoError(err)
		assert.NotEmpty(pullRequests)
	}
}
