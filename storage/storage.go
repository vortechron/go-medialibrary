package storage

import (
	"context"
	"fmt"
	"io"
	"sync"
)


type Storage interface {

	Save(ctx context.Context, path string, contents io.Reader, options ...Option) error


	SaveFromURL(ctx context.Context, path string, url string, options ...Option) error


	Get(ctx context.Context, path string) (io.ReadCloser, error)


	Exists(ctx context.Context, path string) (bool, error)


	Delete(ctx context.Context, path string) error


	URL(path string) string


	TemporaryURL(ctx context.Context, path string, expiry int64) (string, error)
}


type DiskManager struct {
	disks map[string]Storage
	mu    sync.RWMutex
}


func NewDiskManager() *DiskManager {
	return &DiskManager{
		disks: make(map[string]Storage),
	}
}


func (dm *DiskManager) AddDisk(name string, storage Storage) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.disks[name] = storage
}


func (dm *DiskManager) GetDisk(name string) (Storage, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	disk, ok := dm.disks[name]
	if !ok {
		return nil, fmt.Errorf("disk %s not found", name)
	}

	return disk, nil
}


func (dm *DiskManager) HasDisk(name string) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	_, ok := dm.disks[name]
	return ok
}


func (dm *DiskManager) RemoveDisk(name string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	delete(dm.disks, name)
}


type Option func(*Options)


type Options struct {
	ContentType        string
	ContentDisposition string
	Visibility         string
	CacheControl       string
	Metadata           map[string]string
}


func WithContentType(contentType string) Option {
	return func(o *Options) {
		o.ContentType = contentType
	}
}


func WithContentDisposition(contentDisposition string) Option {
	return func(o *Options) {
		o.ContentDisposition = contentDisposition
	}
}


func WithVisibility(visibility string) Option {
	return func(o *Options) {
		o.Visibility = visibility
	}
}


func WithCacheControl(cacheControl string) Option {
	return func(o *Options) {
		o.CacheControl = cacheControl
	}
}


func WithMetadata(metadata map[string]string) Option {
	return func(o *Options) {
		o.Metadata = metadata
	}
}


func NewOptions(opts ...Option) *Options {
	options := &Options{
		Metadata: make(map[string]string),
	}

	for _, opt := range opts {
		opt(options)
	}

	return options
}
