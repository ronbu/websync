package main

import (
	"code.google.com/p/go-html-transform/h5"
	"code.google.com/p/go.net/html"
	"github.com/gorilla/mux"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	zdfHost      = "http://www.zdf.de"
	zdfMediathek = "/ZDFmediathek/"
	zdfRouter    = mux.NewRouter()
	zdfDay       = zdfRouter.NewRoute().
			Path(zdfMediathek+"hauptnavigation/sendung-verpasst/day{d}").
			Queries("flash", "off")
)

func Zdf(f File, fs chan File, es chan error) {
	for i := 0; i <= 7; i++ {
		zdfDayUrl, _ := zdfDay.URL("d", strconv.Itoa(i))
		up, _ := url.Parse(zdfHost + zdfDayUrl.String())
		u := *up
		root, err := grabParse(u)
		if err != nil {
			es <- err
			return
		}

		visited := make(map[url.URL]bool)
		treeContains(root, "div", "class", "row", func(t h5.Tree) {
			treeFind(t, "a", "href", func(t h5.Tree, a html.Attribute) {
				epPath := url.URL{Path: a.Val}
				u := *u.ResolveReference(&epPath)
				if _, ok := visited[u]; ok {
					return
				} else {
					visited[u] = true
				}
				root, err := grabParse(u)
				if err != nil {
					es <- err
					return
				}

				treeFind(root, "a", "href",
					func(t h5.Tree, a html.Attribute) {
						v := a.Val
						if strings.Contains(v, "veryhigh") && // high quality
							strings.Contains(v, "hstreaming.") { // .mov format
							name := filepath.Base(v)

							nf := newFile(f, name, getReaderFn(v))
							nf.Mtime = time.Now()
							fs <- nf
						}
					})
			})
		})
	}
}

func grabParse(u url.URL) (t h5.Tree, err error) {
	r, err := getReaderFn(u.String())()
	if err != nil {
		return
	}
	tp, err := h5.New(r)
	t = *tp
	if err != nil {
		return
	}
	return
}

func treeContains(t h5.Tree, e, key, val string, f func(h5.Tree)) {
	treeFind(t, e, key, func(t h5.Tree, a html.Attribute) {
		f(t)
	})
}

func treeFind(t h5.Tree, e, key string, f func(h5.Tree, html.Attribute)) {
	t.Walk(func(n *html.Node) {
		if a := getAttr(n, key); n.Data == e && a != nil {
			f(h5.NewTree(n), *a)
		}
	})

}

func getAttr(n *html.Node, key string) *html.Attribute {
	for _, a := range n.Attr {
		if key == a.Key {
			return &a
		}
	}
	return nil
}

func parseDate(url string) time.Time {
	r := regexp.MustCompile(`/\d\d/\d\d/(\d\d)(\d\d)(\d\d)`)
	ms := r.FindAllStringSubmatch(url, 1)
	if len(ms) == 1 {
		m := ms[0]
		y, e1 := strconv.Atoi(m[1])
		mo, e2 := strconv.Atoi(m[2])
		d, e3 := strconv.Atoi(m[3])
		if e1 != nil && e2 != nil && e3 != nil {
			return time.Date(y+2000, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
		}
	}
	return time.Now()
}
