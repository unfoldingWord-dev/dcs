// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/util"
)

var (
	_ ObjectStorage = &LocalStorage{}
)

// LocalStorageType is the type descriptor for local storage
const LocalStorageType Type = "local"

// LocalStorageConfig represents the configuration for a local storage
type LocalStorageConfig struct {
	Path string `ini:"PATH"`
}

// LocalStorage represents a local files storage
type LocalStorage struct {
	ctx context.Context
	dir string
}

// NewLocalStorage returns a local files
func NewLocalStorage(ctx context.Context, cfg interface{}) (ObjectStorage, error) {
	configInterface, err := toConfig(LocalStorageConfig{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(LocalStorageConfig)

	if err := os.MkdirAll(config.Path, os.ModePerm); err != nil {
		return nil, err
	}

	return &LocalStorage{
		ctx: ctx,
		dir: config.Path,
	}, nil
}

// Open a file
func (l *LocalStorage) Open(path string) (Object, error) {
	return os.Open(filepath.Join(l.dir, path))
}

// Save a file
func (l *LocalStorage) Save(path string, r io.Reader) (int64, error) {
	p := filepath.Join(l.dir, path)
	if err := os.MkdirAll(filepath.Dir(p), os.ModePerm); err != nil {
		return 0, err
	}

	// always override
	if err := util.Remove(p); err != nil {
		return 0, err
	}

	f, err := os.Create(p)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, r)
}

// Stat returns the info of the file
func (l *LocalStorage) Stat(path string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(l.dir, path))
}

// Delete delete a file
func (l *LocalStorage) Delete(path string) error {
	p := filepath.Join(l.dir, path)
	return util.Remove(p)
}

// URL gets the redirect URL to a file
func (l *LocalStorage) URL(path, name string) (*url.URL, error) {
	return nil, ErrURLNotSupported
}

// IterateObjects iterates across the objects in the local storage
func (l *LocalStorage) IterateObjects(fn func(path string, obj Object) error) error {
	return filepath.Walk(l.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-l.ctx.Done():
			return l.ctx.Err()
		default:
		}
		if path == l.dir {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(l.dir, path)
		if err != nil {
			return err
		}
		obj, err := os.Open(path)
		if err != nil {
			return err
		}
		defer obj.Close()
		return fn(relPath, obj)
	})
}

func init() {
	RegisterStorageType(LocalStorageType, NewLocalStorage)
}
