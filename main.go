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
	"net/url"
	"os"
	"path/filepath"
	"time"
)

var (
	DefaultClient = &http.Client{Jar: cookieJar{}}
	unixZerotime  = time.Unix(0, 0)
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
	fn := FindRemoteFunc(*u)
	if fn == nil {
		err = errors.New("Unknown URL")
		goto Error
	}
	path := flag.Args()[1]
	if _, err = os.Stat(path); err != nil {
		goto Error
	}
	user, pw, err := requireAuth()
	if err != nil {
		goto Error
	}
	if err = Sync(path, fn, *u, DefaultClient, user, pw); err != nil {
		fmt.Println("...")
		goto Error
	}
}

type remoteFunc func(u url.URL, c *http.Client, user, pw string) ([]File, error)

type File struct {
	Path     string
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

func FindRemoteFunc(u url.URL) remoteFunc {
	switch {
	//case u.Host == "elearning.hslu.ch":
	//return Ilias
	case u.Scheme == "dav" || u.Scheme == "davs":
		return Dav
	default:
		return nil
	}
	return nil
}

func Sync(path string, fn remoteFunc, u url.URL,
	client *http.Client, user, pw string) (err error) {
	remotefiles, err := fn(u, client, user, pw)
	if err != nil {
		fmt.Println("...")
		return
	}
	for _, file := range remotefiles {
		fmt.Println("...")
		file.Path = filepath.Join(path, file.Path)
		err = Local(file)
		if err != nil {
			return
		}
	}
	return
}

func Local(file File) (err error) {
	st, err := os.Stat(file.Path)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if st == nil || file.Mtime.After(st.ModTime()) {
		err = os.MkdirAll(filepath.Dir(file.Path), os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}
		osfile, err := os.Create(file.Path)
		if err != nil {
			return err
		}
		defer osfile.Close()
		r, err := file.FileFunc()
		if err != nil {
			return err
		}
		_, err = io.Copy(osfile, r)
		if err != nil {
			return err
		}
		if err = os.Chtimes(file.Path, unixZerotime, unixZerotime); err != nil {
			return err
		}

		osfile.Sync()
		os.Chtimes(file.Path, file.Mtime, file.Mtime)
		log.Println(file.Path)
	}
	return err
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

func requireAuth() (user, password string, err error) {
	if user, err = prompt("User: "); err != nil {
		return
	}
	if password, err = prompt("Password: "); err != nil {
		return
	}
	return
}

type cookieJar map[string]*http.Cookie

func (j cookieJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	for _, cookie := range cookies {
		j[cookie.Name] = cookie
	}
}

func (j cookieJar) Cookies(_ *url.URL) []*http.Cookie {
	var cookies = []*http.Cookie{}
	for _, cookie := range j {
		cookies = append(cookies, cookie)
	}
	log.Print(cookies)
	return cookies
}

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
