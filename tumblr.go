package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mrjones/oauth"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	tumbHost      = "https://api.tumblr.com" //needs to be replaced for testing
	tumbV         = "/v2/"
	tumbFollowing = tumbV + "user/following"
	tumbPosts     = tumbV + "blog/%s/posts?api_key=%s&offset=%d"
)

func Tumblr(f File, files chan File, errs chan error) {
	tumbUri, _ := url.Parse(tumbHost)
	p := f.Url.Path
	if len(p) == 0 || p == "/" {
		tok, _ := OAuth()
		cons := oauth.Consumer{HttpClient: HClient}
		for i := 0; ; i += 20 {
			resp, err := cons.Get(tumbHost+tumbFollowing,
				map[string]string{"offset": strconv.Itoa(i)}, tok)
			check(err)
			var fr followingResponse
			checkResponse(resp, &fr)
			for _, b := range fr.Blogs {
				bUri, err := url.Parse(b.Url)
				check(err)
				bUri.Path = bUri.Host
				files <- File{Url: *bUri}
			}
			if i >= fr.Total_blogs {
				break
			}
		}
	} else {
		if p[len(p)-1] != '/' {
			f.Url.Path += "/"
		}
		tok, _, err := Keychain(*tumbUri)
		if err != nil {
			errs <- err
			return
		}
		tumblrBlog(f, tok, files, errs)
	}
}

func tumblrBlog(fl File, key string, files chan File, errs chan error) {
	for i := int64(0); ; i += 20 {
		blog := strings.Trim(fl.Url.Path, "/")
		u := fmt.Sprintf(tumbHost+tumbPosts, blog, key, i)
		r, err := HClient.Get(u)
		if err != nil {
			errs <- err
			return
		}

		var br blogResponse
		checkResponse(r, &br)

		for _, rawPost := range br.Posts {
			if err := postToFiles(fl, rawPost, files); err != nil {
				errs <- err
			}
		}
		if i >= br.Blog.Posts {
			break
		}
	}
}

func postToFiles(bf File, raw []byte, fs chan File) (err error) {
	var bp basePost
	err = json.Unmarshal(raw, &bp)
	if err != nil {
		return
	}

	bf.Mtime = time.Unix(bp.Timestamp, 0)
	id := strconv.FormatInt(bp.Id, 10)
	var (
		p   interface{}
		ext string
		ff  ReadFn
	)

	switch bp.PostType {
	case "answer", "audio", "chat":
		//not implemented
		return
	case "link":
		lp := linkPost{}
		err = json.Unmarshal(raw, &lp)
		p = lp
		ext = "-link.txt"
		ff = func() (io.ReadCloser, error) { return newRC(lp.Url), nil }
	case "quote":
		lp := quotePost{}
		err = json.Unmarshal(raw, &lp)
		p = lp
		ext = "-quote.txt"
		ff = func() (io.ReadCloser, error) { return newRC(lp.Text), nil }
	case "text":
		lp := textPost{}
		err = json.Unmarshal(raw, &lp)
		p = lp
		ext = ".txt"
		ff = func() (io.ReadCloser, error) { return newRC(lp.Body), nil }
	case "video":
		lp := videoPost{}
		err = json.Unmarshal(raw, &lp)
		p = lp

		pl := lp.Player
		r := regexp.MustCompile(`src="(.+?)" `)
		s := pl[len(pl)-1].Embed_code
		rm := r.FindAllStringSubmatch(s, 1)

		if len(rm) == 1 && len(rm[0]) == 2 {
			vurl := rm[0][1]
			u, _ := url.Parse(vurl)
			if u.Scheme == "" {
				u.Scheme = "http"
			}
			nf := bf
			nf.Url = *u
			//  Is this video stored on tumblr itself?
			if strings.Contains(u.Host, "tumblr.com") {
				// Then we can download the video with a simple GET
				nf.FileFunc = getReaderFn(nf.Url.String())
				fs <- nf
			} else {
				fs <- nf
			}
		} else {
			return errors.New("Videopost: " + s)
		}
		// fmt.Println(lp)
		return // TODO: Metadata  is not saved for videos
	case "photo":
		lp := photoPost{}
		err = json.Unmarshal(raw, &lp)
		p = lp
		for i, photo := range lp.Photos {
			uri := photo.Alt_sizes[0].Url
			ext := uri[len(uri)-4:]
			iname := fmt.Sprintf("%s-%d%s", id, i, ext)
			if len(lp.Photos) == 1 {
				iname = id + ext
			}
			fs <- newFile(bf, iname,
				func() (r io.ReadCloser, err error) {
					resp, err := HClient.Get(uri)
					if err != nil {
						return nil, err
					} else {
						return resp.Body, nil
					}
				})

		}
	default:
		return errors.New("Unknown Tumblr post type")
	}
	if err != nil {
		return err
	}

	if bp.PostType != "photo" {
		fs <- newFile(bf, id+ext, ff)
	}
	// store metadata in json file
	fs <- newFile(bf, "."+id+".json",
		func() (r io.ReadCloser, err error) {
			b, err := json.MarshalIndent(p, "", "\t")
			if err != nil {
				return
			}
			return fakeCloser{bytes.NewReader(b)}, err
		})
	return
}

func checkResponse(rc *http.Response, resp interface{}) {
	if !(rc.StatusCode < 300 && rc.StatusCode >= 200) {
		check(errors.New("Request: " + rc.Status))
	}
	data, err := ioutil.ReadAll(rc.Body)
	check(err)
	var cr completeResponse
	err = json.Unmarshal(data, &cr)
	check(err)
	err = json.Unmarshal(cr.Response, &resp)
	if err != nil {
		println("complete response: ", string(cr.Response))
	}
	check(err)

}

func newFile(f File, p string, rf ReadFn) File {
	f.Url.Path += p
	if rf != nil {
		f.FileFunc = rf
	}
	return f
}

func newRC(s string) io.ReadCloser {
	return fakeCloser{strings.NewReader(s)}
}

type fakeCloser struct {
	io.Reader
}

func (f fakeCloser) Close() (err error) {
	return
}

type completeResponse struct {
	Meta     meta
	Response json.RawMessage
}

type meta struct {
	Status int64
	Msg    string
}

type followingResponse struct {
	Total_blogs int
	Blogs       []followingBlog
}

type followingBlog struct {
	Name, Url string
	Updated   int
}

type blogResponse struct {
	Blog  blog
	Posts []json.RawMessage
}

type blog struct {
	Title       string
	Posts       int64
	Name        string
	Url         string
	Updated     int64
	Description string
	Ask         bool
	Ask_anon    bool
}

type basePost struct {
	Blog_name    string
	Id           int64
	Post_url     string
	PostType     string `json:"type"`
	Timestamp    int64
	Date         string
	Format       string
	Reblog_key   string
	Tags         []string
	Bookmarklet  bool
	Mobile       bool
	Source_url   string
	Source_title string
	Liked        bool
	State        string
	Total_Posts  int64
}

type textPost struct {
	basePost
	Title, Body string
}

type photoPost struct {
	basePost
	Photos        []photoObject
	Caption       string
	Width, Height int64
}

type photoObject struct {
	Caption   string
	Alt_sizes []altSize
}

type altSize struct {
	Width, Height int64
	Url           string
}

type quotePost struct {
	basePost
	Text, Source string
}

type linkPost struct {
	basePost
	Title, Url, Description string
}

type chatPost struct {
	basePost
	Title, Body string
	Dialogue    []dialogue
}

type dialogue struct {
	Name, Label, Phrase string
}

type audioPost struct {
	basePost
	Caption      string
	Player       string
	Plays        int64
	Album_art    string
	Artist       string
	Album        string
	Track_name   string
	Track_number int64
	Year         int64
}

type videoPost struct {
	basePost
	Caption string
	Player  []player
}

type player struct {
	Width      int64
	Embed_code string
}

type answerPost struct {
	basePost
	Asking_name string
	Asking_url  string
	Question    string
	Answer      string
}
