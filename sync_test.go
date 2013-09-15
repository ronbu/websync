package main

import (
	"net/url"
	"os"
	"testing"
	"time"
)

var (
	aFile = File{
		Url:      url.URL{Path: "a"},
		Mtime:    time.Now().Add(-time.Second),
		FileFunc: stringReadFn("test"),
	}
)

func TestLocalNew(t *testing.T) {
	f := aFile
	testLocal(func(tmp string) File {
		aFile.Url.Path = tmp + aFile.Url.Path
		return f
	}, func(err error) {
		checkFile(t, f)
	})
}

func TestLocalOverwrite(t *testing.T) {
	f := aFile
	testLocal(func(tmp string) File {
		f.Url.Path = tmp + f.Url.Path
		Local(f)
		f.Mtime = f.Mtime.Add(time.Second)
		return f
	}, func(err error) {
		checkFile(t, f)
	})
}

func TestLocalNotOverwrite(t *testing.T) {
	f := aFile
	testLocal(func(tmp string) File {
		f.Url.Path = tmp + f.Url.Path
		Local(f)
		nf := f
		nf.Mtime = nf.Mtime.Add(-time.Second)
		return nf
	}, func(err error) {
		checkFile(t, f)
	})
}

// Currently the function will even overwrite directories
// if a new file is added with the same name as the dir
func TestLocalOverwriteDir(t *testing.T) {
	f := aFile
	testLocal(func(tmp string) File {
		f.Url.Path = tmp + f.Url.Path
		os.Mkdir(f.Url.Path, 777)
		return f
	}, func(err error) {
		checkFile(t, f)
	})
}

func TestCreateDirs(t *testing.T) {
	f := aFile
	testLocal(func(tmp string) File {
		f.Url.Path = tmp + "a/dir/oh/uh/hi/ho"
		return f
	}, func(err error) {
		checkFile(t, f)
	})
}

func testLocal(init func(string) File, check func(error)) {
	tmp, rm := TempDir()
	check(Local(init(tmp)))
	rm()
}

func createSomeFile(tmp string) {
	return
}

func checkFile(t *testing.T, f File) {
	st, err := os.Stat(f.Url.Path)

	f.Mtime = removeSubSecond(f.Mtime)

	if err != nil && os.IsNotExist(err) {
		t.Error("File does not exist:", f.Url)
	} else {
		if !(st.ModTime().Equal(f.Mtime)) {
			t.Errorf("Wrong Mtime: %s (%v != %v)", f.Url.Path, f.Mtime, st.ModTime())
		}
	}
}

// OSX does not store time resolutions below seconds
func removeSubSecond(in time.Time) time.Time {
	return time.Date(
		in.Year(),
		in.Month(),
		in.Day(),
		in.Hour(),
		in.Minute(),
		in.Second(),
		0, in.Location())
}
