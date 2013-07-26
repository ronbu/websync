package main

import (
	"errors"
	_ "errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	unixZerotime = time.Unix(0, 0)
)

func main() {
	var err error
Error:
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}

	flag.Usage = func() {
		fmt.Println("URL PATH")
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.PrintDefaults()
		err = errors.New("Wrong number of Arguments")
		goto Error
	}
	u, err := url.Parse(flag.Args()[0])
	if err != nil {
		goto Error
	}
	lookup, err := registry()
	if err != nil {
		goto Error
	}
	fn := lookup(u)
	if fn == nil {
		err = errors.New("Unknown URL")
		goto Error
	}
	path := flag.Args()[1]
	if _, err = os.Stat(path); err != nil {
		goto Error
	}
	user, pw, err := keychainAuth(*u)
	if err != nil {
		println("security: ", err.Error())
	}
	cj, err := cookiejar.New(nil)
	if err != nil {
		goto Error
	}
	c := &http.Client{Jar: cj}
	if err = Sync(path, fn, *u, c, user, pw); err != nil {
		goto Error
	}
}

type remoteFunc func(u url.URL, c *http.Client, user, pw string) ([]File, error)
type fileFunc func() (reader io.ReadCloser, err error)

type File struct {
	Path     string
	Mtime    time.Time
	FileFunc fileFunc
}

func (f File) ReadAll() (content []byte, err error) {
	reader, err := f.FileFunc()
	if err != nil {
		return
	}
	return ioutil.ReadAll(reader)
}

func registry() (find func(u *url.URL) remoteFunc, err error) {
	const (
		HOST = iota
		PROTOCOL
		NAME
	)
	type item struct {
		name string
		kind int
		f    remoteFunc
	}
	items := []item{}
	items = append(items, item{"elearning.hslu.ch", HOST, Ilias})
	items = append(items, item{"api.tumblr.com", HOST, Tumblr})
	items = append(items, item{"youtube.com", HOST, YoutubeDl})
	items = append(items, item{"dav", PROTOCOL, Dav})
	items = append(items, item{"davs", PROTOCOL, Dav})

	c := exec.Command("/usr/bin/env", "youtube-dl", "--extractor-descriptions")
	output, err := c.CombinedOutput()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(output), "\n") {
		items = append(items, item{strings.ToLower(line), NAME, YoutubeDl})
	}

	return func(u *url.URL) remoteFunc {
		for _, item := range items {
			switch item.kind {
			case HOST:
				if strings.HasSuffix(u.Host, item.name) {
					return item.f
				}
			case PROTOCOL:
				if u.Scheme == item.name {
					return item.f
				}
			case NAME:
				if strings.Contains(u.Host, item.name) {
					return item.f
				}
			}
		}
		return nil
	}, nil
}

func Sync(path string, fn remoteFunc, u url.URL,
	client *http.Client, user, pw string) (err error) {
	remotefiles, err := fn(u, client, user, pw)
	if err != nil {
		return
	}
	for _, file := range remotefiles {
		file.Path = filepath.Join(path, file.Path)
		err = Local(file)
		if err != nil {
			return
		}
	}
	return
}

func StripPassword(url url.URL) url.URL {
	url.User = nil
	return url
}

func prompt(msg string) (input string, err error) {
	fmt.Print(msg)
	res := &input
	_, err = fmt.Scanln(res)
	return
}

func requireAuth(u url.URL) (user, password string, err error) {
	user, password, err = keychainAuth(u)
	if err != nil {
		fmt.Println("Error in keychain auth: " + err.Error())
		if user, err = prompt("User: "); err != nil {
			return
		}
		if password, err = prompt("Password: "); err != nil {
			return
		}
	}
	return
}

func keychainAuth(u url.URL) (username, password string, err error) {
	//TODO: Replace this with proper api accessing keychain
	securityCmd := "/usr/bin/security"
	securitySubCmd := "find-internet-password"
	cmd := exec.Command(securityCmd, securitySubCmd, "-gs", u.Host)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "\"acct\"") {
			username = line[18 : len(line)-1]
		}
		if strings.Contains(line, "password: ") {
			password = line[11 : len(line)-1]
		}
	}
	return
}

// type cookieJar map[string]*http.Cookie

// func (j cookieJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
// 	for _, cookie := range cookies {
// 		j[cookie.Name] = cookie
// 	}
// }

// func (j cookieJar) Cookies(_ *url.URL) []*http.Cookie {
// 	var cookies = []*http.Cookie{}
// 	for _, cookie := range j {
// 		cookies = append(cookies, cookie)
// 	}
// 	return cookies
// }

func removeNanoseconds(in time.Time) time.Time {
	return time.Date(
		in.Year(),
		in.Month(),
		in.Day(),
		in.Hour(),
		in.Minute(),
		in.Second(),
		0, in.Location())
}
