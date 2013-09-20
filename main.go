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
	files := Sync(url, path)
	for f := range files {
		if f.Err != nil {
			fmt.Fprintln(os.Stderr, f.Err)
		} else {
			fmt.Println(f.Path)
		}
	}
}
