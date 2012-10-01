package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Local(file File) (err error) {
	st, err := os.Stat(file.Path)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if st == nil || file.Mtime.After(st.ModTime()) {
		err = os.MkdirAll(filepath.Dir(file.Path), os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}
		osfile, err := os.Create(file.Path)
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
		os.Chtimes(file.Path, file.Mtime, file.Mtime)

		osfile.Sync()
		fmt.Println(file.Path)
	}
	return
}
