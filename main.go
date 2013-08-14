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
	cj, err := cookiejar.New(nil)
	if err != nil {
		goto Error
	}
	c := &http.Client{Jar: cj}
	lookup, err := registry(c)
	if err != nil {
		goto Error
	}
	path := flag.Args()[1]
	url := flag.Args()[0]
	if _, err = os.Stat(path); err != nil {
		goto Error
	}
	files, errs := Sync(url, path, lookup)
	for {
		select {
		case f := <-files:
			println(f.Url.Path)
		case e := <-errs:
			fmt.Fprintln(os.Stderr, e)
		}
	}
}

type Remote func(File, *http.Client, string, string, chan File, chan error)
type legacyRemote func(u url.URL, c *http.Client, user, pw string) ([]File, error)

func asyncAdapter(f legacyRemote) Remote {
	return func(fl File, c *http.Client, user, pw string,
		files chan File, errs chan error) {

		fs, err := f(*fl.Url, c, user, pw)
		if err != nil {
			errs <- err
		} else {
			for _, f := range fs {
				files <- f
			}
		}
		return
	}
}

type File struct {
	Url      *url.URL
	Mtime    time.Time
	FileFunc func() (reader io.ReadCloser, err error)
}

func (f File) ReadAll() (content []byte, err error) {
	reader, err := f.FileFunc()
	if err != nil {
		return
	}
	return ioutil.ReadAll(reader)
}

type registryFn func(f File) func(chan File, chan error)

func registry(hc *http.Client) (fun registryFn, err error) {

	const (
		HOST = iota
		PROTOCOL
		NAME
	)
	type item struct {
		name string
		kind int
		f    Remote
	}
	items := []item{}
	items = append(items, item{"elearning.hslu.ch", HOST, asyncAdapter(Ilias)})
	items = append(items, item{"tumblr.com", HOST, Tumblr})
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

	return func(f File) func(chan File, chan error) {
		for _, item := range items {
			match := false
			switch item.kind {
			case HOST:
				if strings.HasSuffix(f.Url.Host, item.name) {
					match = true
				}
			case PROTOCOL:
				if f.Url.Scheme == item.name {
					match = true
				}
			case NAME:
				if strings.Contains(f.Url.Host, item.name) {
					match = true
				}
			}
			if match {
				return func(files chan File, errs chan error) {
					nameUri, _ := url.Parse("http://" + item.name)
					user, password, err := keychainAuth(*nameUri)
					if err != nil {
						errs <- err
					}
					item.f(f, hc, user, password, files, errs)
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

func Sync(from, to string, lookup registryFn) (chan File, chan error) {
	files := make(chan File)
	errs := make(chan error)

	fromUri, err := url.Parse(from)
	if err != nil {
		errs <- err
		return nil, errs
	}

	go func() {
		todos := []File{File{Url: fromUri}}
		for i := 0; i < len(todos); i++ {
			todo := todos[i]

			h := lookup(todo)
			if h == nil {
				errs <- errors.New("Cannot Sync: " + todo.Url.String())
				continue
			}

			hfiles := make(chan File)
			finish := make(chan bool)
			go func(f chan bool) {
				h(hfiles, errs)
				f <- true
			}(finish)

		LOOP:
			for {
				select {
				case <-finish:
					break LOOP
				case f := <-hfiles:
					if f.FileFunc == nil {
						todos = append(todos, f)
					} else {
						f.Url.Path = filepath.Join(to, f.Url.Path)
						err = Local(f)
						if err != nil {
							errs <- err
						} else {
							files <- f
						}
					}
				}
			}
		}
		close(files)
		close(errs)
	}()
	return files, errs
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
		// println(host, name)
		if name == host || name == "www."+host {
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
