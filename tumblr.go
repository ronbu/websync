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
	"strconv"
	"strings"
	"time"
)

const (
	apiVersion   = "/v2/"
	apiFollowing = apiVersion + "user/following"
	apiBlog      = apiVersion + "blog/%s"
	apiPosts     = apiBlog + "/posts?api_key=%s&filter=raw&offset=%d"
)

type tumblr struct {
	DefaultHandler
}

func NewTumblr(c *http.Client, a Auth) Handler {
	return &tumblr{DefaultHandler{c, a}}
}

func (t *tumblr) Url(u *url.URL) (r *url.URL, err error) {
	return u, nil
}
func (t *tumblr) Files(f File, files chan File, errs chan error) {
	token, secret, err := t.OAuth(f.Url)
	check(err)
	tok := &oauth.AccessToken{token, secret}
	cons := oauth.Consumer{HttpClient: t.Client}
	if f.Url.Path != "/" {
		getBlog(f, tok, t.Client, files, errs)
	} else {
		for i := 0; ; i += 20 {
			resp, err := cons.Get(f.Url.String()+apiFollowing,
				map[string]string{"offset": strconv.Itoa(i)}, tok)
			check(err)
			var fr followingResponse
			checkResponse(resp.Body, &fr)
			for _, b := range fr.Blogs {
				bUri, err := url.Parse(b.Url)
				check(err)
				bUri.Path = bUri.Host
				files <- File{Url: bUri}
			}
			if i >= fr.Total_blogs {
				break
			}
		}
	}
}

func checkResponse(rc io.ReadCloser, resp interface{}) {
	data, err := ioutil.ReadAll(rc)
	check(err)
	// println(string(data))
	var cr completeResponse
	err = json.Unmarshal(data, &cr)
	if err != nil || cr.Meta.Status != 200 {
		if cr.Meta.Msg != "" {
			panic(errors.New(cr.Meta.Msg))
		}
	}
	err = json.Unmarshal(cr.Response, &resp)
	if err != nil {
		println(string(cr.Response))
	}
	check(err)
}

func getBlog(fl File, key *oauth.AccessToken, c *http.Client,
	files chan File, errs chan error) {

	for i := int64(0); ; i += 20 {
		thost := fl.Url.Path[1:]
		fl.Url.Path = ""
		u := fmt.Sprintf(fl.Url.String()+apiPosts, thost, key.Token, i)
		r, err := c.Get(u)
		if err != nil {
			errs <- err
			return
		}

		var br blogResponse
		checkResponse(r.Body, &br)

		for _, rawPost := range br.Posts {
			var p post
			err = json.Unmarshal(rawPost, &p)
			if err != nil {
				errs <- err
				return
			}

			fileName := strconv.FormatInt(p.Id, 10)
			mtime := time.Unix(p.Timestamp, 0)

			//store metadata
			files <- File{
				Url:   &url.URL{Path: fl.Url.Path + "/" + fmt.Sprintf(".%s.json", fileName)},
				Mtime: mtime,
				FileFunc: func() (r io.ReadCloser, err error) {
					b, err := json.MarshalIndent(p, "", "\t")
					if err != nil {
						return
					}
					// println(string(b))
					return fakeCloser{bytes.NewReader(b)}, err
				},
			}
			switch p.PostType {
			case "answer", "audio", "chat":
				//not implemented
				continue
			case "link":
				var p linkPost
				err = json.Unmarshal(rawPost, &p)
				if err != nil {
					errs <- err
					return
				}
				files <- File{
					Url: &url.URL{Path: fl.Url.Path + "/" + fmt.Sprintf(
						"%d_link.txt", p.Id)},
					Mtime: mtime,
					FileFunc: func() (r io.ReadCloser, err error) {
						return fakeCloser{strings.NewReader(p.Url)}, nil
					},
				}
			case "quote":
				var p quotePost
				err = json.Unmarshal(rawPost, &p)
				if err != nil {
					errs <- err
					return
				}
				files <- File{
					Url: &url.URL{Path: fl.Url.Path + "/" + fmt.Sprintf(
						"%d_quote.txt", p.Id)},
					Mtime: mtime,
					FileFunc: func() (r io.ReadCloser, err error) {
						return fakeCloser{strings.NewReader(p.Text)}, nil
					},
				}

			case "text":
				var p textPost
				err = json.Unmarshal(rawPost, &p)
				if err != nil {
					errs <- err
					return
				}
				files <- File{
					Url: &url.URL{Path: fl.Url.Path + "/" + fmt.Sprintf(
						"%d.md", p.Id)},
					Mtime: mtime,
					FileFunc: func() (r io.ReadCloser, err error) {
						return fakeCloser{strings.NewReader(p.Body)}, nil
					},
				}

			case "video":
				// println("video source: ", p.Source_url)
				// println("post url: ", p.Post_url)
				// TODO: Fix
				// u, _ := url.Parse(p.)
				// files <- File{Url: u}
				continue
			case "photo":
				var p photoPost
				err = json.Unmarshal(rawPost, &p)
				if err != nil {
					errs <- err
					return
				}

				for i, photo := range p.Photos {
					uri := photo.Alt_sizes[0].Url
					files <- File{
						Url: &url.URL{Path: fl.Url.Path + "/" + fmt.Sprintf(
							"%s-%d.%s", fileName, i, uri[len(uri)-3:])},
						Mtime: mtime,
						FileFunc: func() (
							r io.ReadCloser, err error) {
							resp, err := c.Get(uri)
							if err != nil {
								return nil, err
							} else {
								return resp.Body, nil
							}
						},
					}
				}
				continue
			default:
				errs <- errors.New("Do not know this type")
				return

			}
		}
		if i >= br.Blog.Posts {
			break
		}
	}
	return
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
