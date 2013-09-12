package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const (
	apiPostsRoute = apiVersion + "blog/{blog}/"
)

func tumblrSimulator(blogs ...blogResponse) *httptest.Server {
	r := mux.NewRouter()
	r.NewRoute().
		Path(apiPostsRoute).
		Queries("api_key", "", "offset", `[0-9]+`).
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		rb := vars["blog"]
		for _, b := range blogs {
			if b.Blog.Name == rb {
				enc := json.NewEncoder(w)
				mb, err := json.Marshal(b)
				check(err)
				cr := completeResponse{meta{200, "OK"}, mb}
				check(enc.Encode(cr))
			}
		}
	})
	return httptest.NewServer(r)
}

func TestTumblr(t *testing.T) {

	s := tumblrSimulator()
	defer s.Close()

	tumb := NewTumblr(http.DefaultClient, &fakeAuth{"u", "s"})
	files := make(chan File)
	errs := make(chan error)
	u, _ := url.Parse(s.URL + "/" + "blog")
	tumb.Files(File{Url: u}, files, errs)
}
