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
	"regexp"
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
type asyncRemote func(u url.URL, c *http.Client, user, pw string,
	files chan File, errs chan error)
type fileFunc func() (reader io.ReadCloser, err error)

type File struct {
	Url      *url.URL
	Mtime    time.Time
	FileFunc fileFunc
}

func asyncAdapter(f remoteFunc) asyncRemote {
	return func(u url.URL, c *http.Client, user, pw string,
		files chan File, errs chan error) {

		fs, err := f(u, c, user, pw)
		if err != nil {
			errs <- err
		} else {
			for _, f := range fs {
				files <- f
			}
		}
		close(errs)
		close(files)
		return
	}
}

func (f File) ReadAll() (content []byte, err error) {
	reader, err := f.FileFunc()
	if err != nil {
		return
	}
	return ioutil.ReadAll(reader)
}

func registry() (find func(u *url.URL) asyncRemote, err error) {
	const (
		HOST = iota
		PROTOCOL
		NAME
	)
	type item struct {
		name string
		kind int
		f    asyncRemote
	}
	items := []item{}
	items = append(items, item{"elearning.hslu.ch", HOST, asyncAdapter(Ilias)})
	items = append(items, item{"tumblr.com", HOST, asyncAdapter(Tumblr)})
	items = append(items, item{"dav", PROTOCOL, asyncAdapter(Dav)})
	items = append(items, item{"davs", PROTOCOL, asyncAdapter(Dav)})

	c := exec.Command("/usr/bin/env", "youtube-dl", "--extractor-descriptions")
	output, err := c.CombinedOutput()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(output), "\n") {
		items = append(items, item{strings.ToLower(line), NAME, asyncAdapter(YoutubeDl)})
	}

	return func(u *url.URL) asyncRemote {
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

// func run(c exec.Cmd){
// 		b, err := cmd.CombinedOutput()
// 	output := string(b)
// 	if err != nil {
// 		if output == "" {
// 			return
// 		} else {
// 			output = strings.Replace(output, "\n", "\t", -1)
// 			return nil, errors.New(
// 				strings.Join(cmd.Args, " ") + ": \n" + output)
// 		}
// 	}
// }

func Sync(path string, fn asyncRemote, u url.URL,
	client *http.Client, user, pw string) (err error) {
	errs := make(chan error)
	files := make(chan File, 1)

	go fn(u, client, user, pw, files, errs)

	sync := make(chan bool, 5)
	for {
		select {
		case f, ok := <-files:
			if !ok {
				return
			}
			sync <- true
			go func() {
				f.Url.Path = filepath.Join(path, f.Url.Path)
				err = Local(f)
				if err != nil {
					return
				}
				<-sync
			}()
		case err = <-errs:
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
	host := findHost(u.Host)
	if host == "" {
		err = errors.New("No Keychain item found")
		return
	}

	securityCmd := "/usr/bin/security"
	securitySubCmd := "find-internet-password"
	cmd := exec.Command(securityCmd, securitySubCmd, "-gs", host)
	b, err := cmd.CombinedOutput()
	output := string(b)
	if err != nil {
		return
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "\"acct\"") {
			username = line[18 : len(line)-1]
		}
		if strings.Contains(line, "password: ") {
			password = line[11 : len(line)-1]
		}
	}
	return
}

func findHost(host string) (result string) {
	securityCmd := "/usr/bin/security"
	securitySubCmd := "dump-keychain"
	cmd := exec.Command(securityCmd, securitySubCmd)
	b, err := cmd.CombinedOutput()
	out := string(b)
	if err != nil {
		return
	}
	r := regexp.MustCompile(`srvr"<blob>="(.*?)"`)
	ms := r.FindAllStringSubmatch(out, -1)
	for _, m := range ms {
		name := m[1]
		// println(name)
		if strings.HasSuffix(name, host) || strings.HasSuffix(host, name) {
			// println(name)
			return name
		}
	}
	return ""
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
