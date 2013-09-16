package main

import (
	"errors"
	_ "errors"
	"flag"
	"fmt"
	"log"
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
	path := flag.Args()[1]
	url := flag.Args()[0]
	if _, err = os.Stat(path); err != nil {
		goto Error
	}
	files, errs := Sync(url, path, Lookup)
	for {
		select {
		case f, ok := <-files:
			if !ok {
				return
			}
			println(f.Path)
		case e := <-errs:
			fmt.Fprintln(os.Stderr, e)
		}
	}
}
