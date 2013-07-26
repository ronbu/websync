package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	api           = "http://api.tumblr.com/v2"
	apiBlog       = api + "/blog/%s.tumblr.com"
	apiPhotoPosts = apiBlog + "/posts?api_key=%s&filter=raw&offset=%d"
)

type fakeCloser struct {
	io.Reader
}

func (f fakeCloser) Close() (err error) {
	return
}

func Tumblr(u url.URL, c *http.Client, _, key string) (
	files []File, err error) {
	for i := int64(0); i < 150; i += 20 {
		reqUrl := apiPhotoPosts
		r, err := c.Get(fmt.Sprintf(reqUrl, u.Path[1:], key, i))
		if err != nil {
			return files, err
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return files, err
		}
		println(string(r.Request.URL.String()))
		// println(string(body))
		var cr completeResponse
		err = json.Unmarshal(body, &cr)
		if err != nil {
			if cr.Meta.Msg != "" {
				err = errors.New(u.String() + ": " + cr.Meta.Msg)
			}
			return files, err
		}

		for _, rawPost := range cr.Response.Posts {
			var p post
			err = json.Unmarshal(rawPost, &p)
			if err != nil {
				return nil, err
			}

			fileName := strconv.FormatInt(p.Id, 10)
			mtime := time.Unix(p.Timestamp, 0)

			//store metadata
			files = append(files, File{
				Path:  fmt.Sprintf(".%s.json", fileName),
				Mtime: mtime,
				FileFunc: func() (r io.ReadCloser, err error) {
					b, err := json.MarshalIndent(p, "", "\t")
					if err != nil {
						return
					}
					// println(string(b))
					return fakeCloser{bytes.NewReader(b)}, err
				},
			})

			switch p.PostType {
			case "quote", "link", "answer", "audio", "chat":
				//not implemented
				continue

			case "video":
				// println("video source: ", p.Source_url)
				// println("post url: ", p.Post_url)
				u, _ := url.Parse(p.Source_url)
				f, err := YoutubeDl(*u, c, "", "")
				if err != nil || len(f) != 1 {
					continue
				}
				files = append(files, f[0])

			case "text":
				var p textPost
				err = json.Unmarshal(rawPost, &p)
				if err != nil {
					return nil, err
				}
				files = append(files, File{
					Path: fmt.Sprintf(
						"%d.md", i),
					Mtime: mtime,
					FileFunc: func() (r io.ReadCloser, err error) {
						return fakeCloser{strings.NewReader(p.Body)}, nil
					},
				})

			case "photo":
				var p photoPost
				err = json.Unmarshal(rawPost, &p)
				if err != nil {
					return nil, err
				}

				for i, photo := range p.Photos {
					url := photo.Alt_sizes[0].Url
					files = append(files, File{
						Path: fmt.Sprintf(
							"%s-%d.%s", fileName, i, url[len(url)-3:]),
						Mtime: mtime,
						FileFunc: func() (
							r io.ReadCloser, err error) {
							resp, err := c.Get(url)
							return resp.Body, err
						},
					})
				}
			default:
				return nil, err

			}
		}
		if i > cr.Response.Blog.Posts {
			break
		}
	}
	return
}

type completeResponse struct {
	Meta     meta
	Response response
}

type meta struct {
	Status int64
	Msg    string
}

type response struct {
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

type post struct {
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
	post
	Title, Body string
}

type photoPost struct {
	post
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
	post
	Text, Source string
}

type linkPost struct {
	post
	Title, Url, Description string
}

type chatPost struct {
	post
	Title, Body string
	Dialogue    []dialogue
}

type dialogue struct {
	Name, Label, Phrase string
}

type audioPost struct {
	post
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
	post
	Caption string
	Player  []player
}

type player struct {
	Width      int64
	Embed_code string
}

type answerPost struct {
	post
	Asking_name string
	Asking_url  string
	Question    string
	Answer      string
}
