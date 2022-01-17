package store

import (
	"fmt"
	"strings"

	"github.com/peterbourgon/diskv/v3"
)

const (
	CACHE_PREFIX = ".disk-cache"
)

func folderTransform(key string) *diskv.PathKey {
	path := strings.Split(key, "/")
	last := len(path) - 1
	return &diskv.PathKey{
		Path:     path[:last],
		FileName: path[last] + ".json",
	}
}

func inverseFolderTransform(pathKey *diskv.PathKey) (key string) {
	j := pathKey.FileName[len(pathKey.FileName)-4:]
	if j != ".json" {
		panic("Invalid file found in storage folder!")
	}
	return strings.Join(pathKey.Path, "/") + pathKey.FileName[:len(pathKey.FileName)-5]
}

type Disk struct {
	diskv *diskv.Diskv
}

func (d *Disk) Get(key string) ([]byte, error) {
	if !d.diskv.Has(key) {
		return nil, ErrNotFound
	}

	return d.diskv.Read(key)
}

func (d *Disk) Put(key string, value []byte) error {
	err := d.diskv.Write(key, value)
	if err != nil {
		return ErrInternalPutFailed{
			key:   key,
			value: value,
			err:   err,
		}
	}
	return nil
}

func NewDisk(owner string, repo string) *Disk {
	return &Disk{
		diskv: diskv.New(
			diskv.Options{
				BasePath:          fmt.Sprintf("%s/%s/%s", CACHE_PREFIX, owner, repo),
				AdvancedTransform: folderTransform,
				InverseTransform:  inverseFolderTransform,
			},
		),
	}
}
