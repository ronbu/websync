package main

import (
	"io"
	"os"
	"path/filepath"
)

func Local(file File) (err error) {
	st, err := os.Stat(file.Url.Path)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if st == nil || file.Mtime.After(st.ModTime()) {
		err = os.MkdirAll(filepath.Dir(file.Url.Path), os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}
		osfile, err := os.Create(file.Url.Path)
		if err != nil {
			return err
		}
		defer osfile.Close()
		r, err := file.FileFunc()
		if err != nil {
			return err
		}
		_, err = io.Copy(osfile, r)
		if err != nil {
			return err
		}
		os.Chtimes(file.Url.Path, file.Mtime, file.Mtime)

		osfile.Sync()
	}
	return
}
