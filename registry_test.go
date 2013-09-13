package main

import (
	"testing"
	"time"
)

func listIndexFn(t *testing.T, ifn IndexFn, f File, fs chan File, es chan error) (files []File) {
	go func() {
		ifn(f, fs, es)
	}()
	for {
		select {
		case f := <-fs:
			files = append(files, f)
		case e := <-es:
			if e != nil {
				t.Fatal(e)
			}
		case <-time.After(1 * time.Second):
			return
		}
	}
}
