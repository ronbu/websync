package main

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func NewDavHandler(fs testFS, t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO check for basic authentication
		switch r.Method {
		case "PROPFIND":
			resp := Multistatus{Responses: []Response{}}
			for path, file := range fs {
				var mimetype string
				// if file.isDir {
				// 	mimetype = "httpd/unix-directory"
				// } else {
				mimetype = "text/plain"
				// }
				resp.Responses = append(resp.Responses, Response{
					Name:     path,
					Mtime:    file.Mtime.Format(time.RFC1123),
					Mimetype: mimetype,
				})
			}
			b, err := xml.Marshal(resp)
			if err != nil {
				t.Fatal(err)
			}
			w.Write(b)
		case "GET":
			if file, ok := fs[r.URL.Path]; ok {
				content, err := file.ReadAll()
				if err != nil {
					t.Fatal(err)
				}
				w.Write([]byte(content))
			} else {
				t.Fatal("requeset to: ", r.URL.Path)
			}
		}
	})
}

func TestDav(t *testing.T) {
	//TODO: Cleanup this function most of it is not needed anymore
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO check for basic authentication
		switch r.Method {
		case "PROPFIND":
			b, err := xml.Marshal(Multistatus{Responses: []Response{
				{
					Name:     "/a/b/c",
					Mtime:    "Thu, 29 Mar 2012 20:38:59 GMT",
					Mimetype: "httpd/unix-directory",
				}, {
					Name:     "/a/b/c/d",
					Mtime:    "Thu, 29 Mar 2012 20:38:59 GMT",
					Mimetype: "text/plain",
				},
			}})
			if err != nil {
				t.Fatal(err)
			}
			w.Write(b)
		case "GET":
			if r.URL.Path == "/a/b/c/d" {
				w.Write([]byte("filecontent"))
			} else {
				t.Fatal("requeset to: ", r.URL.Path)
			}
		}
	}))
	defer server.Close()
	client := http.DefaultClient
	baseurl, _ := url.Parse(server.URL + "/a/b")
	files, err := Dav(*baseurl, client, "u", "p")
	if err != nil {
		t.Fatal(err)
	}
	//t.Fatal(len(files))
	location := time.FixedZone("GMT", 0)
	r2 := files[0]
	e2url := *baseurl
	e2url.Path += "/c/d"
	e2 := File{
		Url:   &url.URL{Path: "/c/d"},
		Mtime: time.Date(2012, 3, 29, 20, 38, 59, 0, location),
	}
	if r2.Url.Path != e2.Url.Path || !r2.Mtime.Equal(e2.Mtime) {
		t.Fatal(r2)
	}
}
