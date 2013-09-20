package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mrjones/oauth"
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

func Tumblr(f File, ch chan File) {
	tumbUri, _ := url.Parse(tumbHost)
	p := f.Url.Path
	if len(p) == 0 || p == "/" {
		tok, err := OAuth()
		if err != nil {
			f.SendErr(ch, &err)
			return
		}
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
				ch <- File{Url: *bUri}
			}
			if i >= fr.Total_blogs {
				break
			}
		}
	} else {
		tok, _, err := Keychain(*tumbUri)
		if err != nil {
			f.SendErr(ch, &err)
			return
		}
		err = tumblrBlog(f, tok, ch)
		f.SendErr(ch, &err)
	}
	return
}

func tumblrBlog(fl File, key string, files chan File) (err error) {
	for i := int64(0); ; i += 20 {
		// println(fl.Url.String())
		blog := strings.Trim(fl.Url.Path, "/")
		u := fmt.Sprintf(tumbHost+tumbPosts, blog, key, i)
		var r *http.Response
		r, err = HClient.Get(u)
		if err != nil {
			return
		}

		var br blogResponse
		checkResponse(r, &br)

		for _, rawPost := range br.Posts {
			tumblrPost(fl, rawPost, files)
		}
		if i >= br.Blog.Posts {
			break
		}
	}
	return nil
}

func tumblrPost(f File, raw []byte, ch chan File) {
	var bp basePost
	err := json.Unmarshal(raw, &bp)
	if err != nil {
		f.SendErr(ch, &err)
		return
	}
	f.Mtime = time.Unix(bp.Timestamp, 0)
	id := strconv.FormatInt(bp.Id, 10)
	f.Path += id
	var p interface{}

	ptype := bp.PostType
	if ptype == "photo" {
		p = tumblrPhoto(f, raw, ch)
	} else {
		if ptype == "video" {
			f, p = tumblrVideo(f, raw)
		} else {
			f, p = tumblrText(f, raw, ptype)
		}
		ch <- f
	}
	// store metadata in json file
	f.Path += ".json"
	b, err := json.MarshalIndent(p, "", "\t")
	f.Err = err
	f.Body = b
	if f.Err == nil {
		f.SetLeaf()
	}
	ch <- f.SetLeaf()
}

func tumblrVideo(f File, raw []byte) (File, videoPost) {
	p := videoPost{}
	err := json.Unmarshal(raw, &p)
	if err != nil {
		f.Err = err
		return f, p
	}

	pl := p.Player
	r := regexp.MustCompile(`src="(.+?)" `)
	s := pl[len(pl)-1].Embed_code
	rm := r.FindAllStringSubmatch(s, 1)

	if len(rm) == 1 && len(rm[0]) == 2 {
		vurl := rm[0][1]
		u, _ := url.Parse(vurl)
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		f.Url = *u
		if strings.HasSuffix(f.Url.Host, "tumblr.com") {
			f = f.SetLeaf()
		}
	} else {
		f.Err = errors.New("Videopost: " + s)
	}
	f.Path += "-"
	return f, p
}

func tumblrPhoto(f File, raw []byte, ch chan File) photoPost {
	p := photoPost{}
	f.Err = json.Unmarshal(raw, &p)
	for i, photo := range p.Photos {
		us := photo.Alt_sizes[0].Url
		u, err := url.Parse(us)
		nf := f
		if len(p.Photos) != 1 {
			nf.Path += "-" + strconv.Itoa(i)
		}
		if err != nil {
			nf.Err = err
		} else {
			nf.Url = *u
		}
		ch <- nf.SetLeaf()
	}
	return p
}

func tumblrText(bf File, raw []byte, t string) (File, interface{}) {
	var err error
	var p interface{}
	var ext, content string

	switch t {
	case "answer", "audio", "chat":
		//not implemented
	case "link":
		lp := linkPost{}
		err = json.Unmarshal(raw, &lp)
		p = lp
		ext = "-link.txt"
		content = lp.Url
	case "quote":
		lp := quotePost{}
		err = json.Unmarshal(raw, &lp)
		p = lp
		ext = "-quote.txt"
		content = lp.Text
	case "text":
		lp := textPost{}
		err = json.Unmarshal(raw, &lp)
		p = lp
		ext = ".txt"
		content = lp.Body
	default:
		bf.Err = errors.New("Unknown Tumblr post type")
	}
	bf.Body = []byte(content)
	bf.Path += ext
	if err == nil {
		bf = bf.SetLeaf()
	} else {
		bf.Err = err
	}

	return bf, p
}

func checkResponse(rc *http.Response, resp interface{}) {
	if !(rc.StatusCode < 300 && rc.StatusCode >= 200) {
		println(rc.Request.URL.String())
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
