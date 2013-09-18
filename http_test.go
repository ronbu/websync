package main

import (
	"net/http"
	// "net/url"
	"strings"
	"testing"
	"time"
)

func TestHttp(t *testing.T) {
	content := "content"
	now := removeSubSecond(time.Now())
	// never := time.Time{}

	bf := File{Mtime: now}
	bf = bf.FromString(content)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		println(r.RequestURI)
		http.ServeContent(w, r, "somename", now, strings.NewReader(content))
	})

	testIndex(t, &httpHost, Http, h, convertTestcases([]File{
		File{}, bf,
		File{path: "a"}, NewFile(bf, "a.txt", nil, nil, nil),
	}))
}

func convertTestcases(files []File) []testcase {
	res := []testcase{}
	for i := 0; i < len(files); i += 2 {
		res = append(res, testcase{in: files[i], exp: []File{files[i+1]}})
	}
	return res
}
