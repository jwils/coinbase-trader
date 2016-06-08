package storage

import "sync"

const (
	META_FILE_NAME = "meta"
)

type DB struct {
	path       string
	sources    map[string]string
	sourceLock sync.RWMutex
}
