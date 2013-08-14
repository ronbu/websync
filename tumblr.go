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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	api           = "http://api.tumblr.com/v2"
	apiBlog       = api + "/blog/%s"
	apiPhotoPosts = apiBlog + "/posts?api_key=%s&filter=raw&offset=%d"
)

type fakeCloser struct {
	io.Reader
}

func (f fakeCloser) Close() (err error) {
	return
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
	check(err)
}

func Tumblr(f File, c *http.Client, user, pass string,
	files chan File, errs chan error) {
	tumbUri, _ := url.Parse("http://api.tumblr.com")
	key, secret, err := keychainAuth(*tumbUri)
	check(err)
	if f.Url.Host != "tumblr.com" {
		getBlog(f, key, c, files, errs)
	} else {
		cons := oauth.NewConsumer(key, secret, oauth.ServiceProvider{
			RequestTokenUrl:   "http://www.tumblr.com/oauth/request_token",
			AuthorizeTokenUrl: "https://www.tumblr.com/oauth/authorize",
			AccessTokenUrl:    "https://www.tumblr.com/oauth/access_token",
		})
		// cons.Debug(true)
		requestToken, userUri, err := cons.GetRequestTokenAndUrl("https://localhost")
		check(err)
		// println(userUri)
		verifier := OAuth(userUri, user, pass)
		// println(verifier)
		token, err := cons.AuthorizeToken(requestToken, verifier)
		check(err)
		for i := 0; ; i += 20 {
			resp, err := cons.Get("https://api.tumblr.com/v2/user/following",
				map[string]string{"offset": strconv.Itoa(i)}, token)
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

func OAuth(uri, user, pass string) (redirect string) {
	js := `var casper = require("casper").create({
    // verbose: true,
    // logLevel: "debug"
});

casper.start(casper.cli.args[0], function() {
	// TODO: Transfer user and password through stdin
	this.fill("form#signup_form",{
		"user[email]":    casper.cli.args[1],
		"user[password]": casper.cli.args[2]
	}, true);
});

casper.then(function(){
	this.mouseEvent('click', 'button[name=allow]')
});

casper.then(function(response){
	console.log(response.url);
});

casper.run();`
	base, rm := TempDir()
	defer rm()
	jspath := filepath.Join(base, "oauth.js")
	err := ioutil.WriteFile(jspath, []byte(js), 0777)
	check(err)
	c := exec.Command("/usr/bin/env", "casperjs", jspath, uri, user, pass)
	out, _ := c.CombinedOutput()
	check(err)
	u, err := url.Parse(string(out))
	check(err)
	verifier := u.Query().Get("oauth_verifier")
	return verifier
}

func getBlog(fl File, key string, c *http.Client,
	files chan File, errs chan error) {

	for i := int64(0); ; i += 20 {
		reqUrl := apiPhotoPosts
		u := fmt.Sprintf(reqUrl, fl.Url.Host, key, i)
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
