// Package jsonstore keeps one JSON document on disk. It is read whole, written whole, and put in
// place by a rename, so a crash mid-write leaves the document that was there rather than half of
// a new one. The connection profiles and the query history are each one of these.
package jsonstore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

const (
	fileMode = 0o600
	dirMode  = 0o700
)

// File is the document at one path. A file that is not there reads as the zero document, which is
// what an app that has never saved anything means.
type File[T any] struct {
	path string
	mu   sync.Mutex
}

func New[T any](path string) *File[T] {
	return &File[T]{path: path}
}

// Path is where a document of this name lives: one directory of ours inside the reader's own
// configuration directory, so everything the app keeps sits together.
func Path(name string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vibe-db", name), nil
}

func (f *File[T]) Read() (T, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.read()
}

// Update reads the document, hands it to change, and writes it back — all under one lock, so two
// callers cannot each read the same document and write the other's away. A change that fails
// writes nothing, because a rejected edit has to leave the file exactly as it was.
func (f *File[T]) Update(change func(*T) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	doc, err := f.read()
	if err != nil {
		return err
	}
	if err := change(&doc); err != nil {
		return err
	}
	return f.write(doc)
}

func (f *File[T]) read() (T, error) {
	var doc T

	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return doc, err
	}

	if err := json.Unmarshal(data, &doc); err != nil {
		var zero T
		return zero, err
	}
	return doc, nil
}

func (f *File[T]) write(doc T) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(f.path)+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if err := tmp.Chmod(fileMode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, f.path)
}
