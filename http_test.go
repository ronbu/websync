package main

import (
	"net/http"
	"testing"
	"time"
)

func TestHttp(t *testing.T) {
	mtime := time.Now()
	bf := NewFile(File{}, "", nil, &mtime, nil).FromString("content")

	basic := "basic.txt"
	normal := "a.txt"

	testIndex(t, &httpHost, Http, []testCase{
		{in: bf.Append(basic), exp: []File{bf.Append(basic)}},
		{in: bf.Append(normal), exp: []File{bf.Append(normal)}},
		{in: bf.Append("txt"), exp: []File{bf.Append("txt.txt")}},
		{in: bf.Append("html"), exp: []File{bf.Append("html.html")}},
		{in: bf.Append("asp"), exp: []File{bf.Append("asp.asp")}},
	}, func(fs []File) http.Handler {
		f := fs[0]
		c, _ := f.ReadAll()
		auth := true
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			println(r.URL.String())
			switch r.URL.Path[1:] {
			case basic:
				// TODO: implement basic authentication
				if auth {
					w.WriteHeader(http.StatusUnauthorized)
				}
				auth = false
				fallthrough
			default:
				w.Write(c)
			}
		})
	})

}
