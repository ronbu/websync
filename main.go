package main

import (
	"errors"
	_ "errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
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
	a := &auth{}
	lookup, err := Registry(c, a)
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
		case f, ok := <-files:
			if !ok {
				return
			}
			println(f.Url.Path)
		case e := <-errs:
			fmt.Fprintln(os.Stderr, e)
		}
	}
}
