package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	api           = "http://api.tumblr.com/v2"
	apiBlog       = api + "/blog/%s"
	apiPhotoPosts = apiBlog + "/posts?api_key=%s&filter=raw&offset=%d"
)

func Tumblr(u url.URL, c *http.Client, _, key string) (
	files []File, err error) {
	for i := 0; i < 100; i += 20{
		reqUrl := apiPhotoPosts
		r, err := c.Get(fmt.Sprintf(reqUrl, u.Path[1:], key, i))
		if err != nil {
			return files, err
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return files, err
		}
		var cr completeResponse
		err = json.Unmarshal(body, &cr)
		if err != nil {
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

			switch p.PostType {
			case "text", "quote", "link", "answer", "video", "audio", "chat":
				//not implemented
				continue
			case "photo":
				// TODO handle Photosets correctly
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
