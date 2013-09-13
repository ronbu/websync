package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

var (
	tumbPostsRoute = tumbV + "blog/{blog}/posts"
)

func simulateTumblr(t *testing.T, b blog, posts ...interface{}) *httptest.Server {
	b.Posts = int64(len(posts))
	var br blogResponse
	for _, p := range posts {
		mp, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		br.Posts = append(br.Posts, mp)
	}
	mbr, err := json.Marshal(br)
	if err != nil {
		t.Fatal(err)
	}

	r := mux.NewRouter()
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal(r.URL)
		b, err := json.Marshal(completeResponse{meta{404, "Not Found"}, []byte{}})
		check(err)
		http.Error(w, string(b), 404)
	})
	r.NewRoute().
		Name("posts").
		Path(tumbPostsRoute).
		Queries("api_key", "", "offset", ``, "filter", "").
		HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			println(r.URL)
			vars := mux.Vars(r)
			rb := vars["blog"]
			if b.Name == rb {
				cr := completeResponse{Meta: meta{200, "OK"}, Response: mbr}
				if err := json.NewEncoder(w).Encode(cr); err != nil {
					t.Fatal(err)
				}
			}
		})
	// req, _ := http.NewRequest("GET", apiPostsRoute, bytes.NewReader([]byte{}))
	// t.Fatal(r.Get("posts").Match(req, &mux.RouteMatch{}))
	return httptest.NewServer(r)
}

func TestPost(t *testing.T) {
	b := blog{Name: "blog"}

	p := post{
		Id:        42,
		Timestamp: 42,
		PostType:  "text",
	}

	s := simulateTumblr(t, b, p)
	defer s.Close()
	tumbHost = s.URL

	tumb := NewTumblr(http.DefaultClient, &fakeAuth{"u", "s"})
	files := make(chan File)
	errs := make(chan error)
	u, _ := url.Parse(s.URL + "/" + "blog")
	fs := readHandler(t, tumb, File{Url: u}, files, errs)
	t.Log(fs)
}

func readHandler(t *testing.T, h Handler, f File, fs chan File, es chan error) (files []File) {
	go func() {
		h.Files(f, fs, es)
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
