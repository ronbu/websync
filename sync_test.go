package main

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestRecursive(t *testing.T) {
	// Recursive is depth first
	input := [][]int{{1, 0, 6}, {0, 4, 5}, {2, 3}}
	ch := make(chan File)
	go func() {
		recursive(File{}, ch, fakeIndex(input))
		close(ch)
	}()

	expN := 1
LOOP:
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				break LOOP
			}
			n, _ := strconv.Atoi(f.Path)
			if n != expN {
				t.Errorf("%d should have been %d", n, expN)
			}
			expN++
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout expected:", expN)
			break LOOP
		}
	}
	expected := flattenInts(input)
	if expN < len(expected) {
		t.Error("Missing:", expected[expN:])
	}
}

func fakeIndex(nums [][]int) IndexFn {
	i := 0
	return func(f File, ch chan File) {
		for _, n := range nums[i] {
			nf := f
			nf.Path = strconv.Itoa(n)
			if n == 0 {
				i++
			} else {
				nf = nf.SetLeaf()
			}
			ch <- nf
		}
	}
}

func flattenInts(ints [][]int) []int {
	res := []int{}
	for _, g := range ints {
		for _, n := range g {
			if n == 0 {
				continue
			}
			res = append(res, n)
		}
	}
	return res
}

func TestLocalNew(t *testing.T) {
	testLocal(t, func(f File, r io.ReadCloser) File {
		return f
	}, true)
}

func TestLocalOverwriteOlder(t *testing.T) {
	testLocal(t, func(f File, r io.ReadCloser) File {
		err := local(f, r)
		if err != nil {
			t.Error(err)
		}
		f.Mtime = f.Mtime.Add(time.Second)
		return f
	}, true)
}

func TestLocalNotOverwriteNewer(t *testing.T) {
	testLocal(t, func(f File, r io.ReadCloser) File {
		err := local(f, r)
		if err != nil {
			t.Error(err)
		}
		f.Mtime = f.Mtime.Add(-time.Second)
		return f
	}, false)
}

func TestLocalCreateDirs(t *testing.T) {
	testLocal(t, func(f File, r io.ReadCloser) File {
		f.Path += "a/dir/oh/uh/hi/ho"
		return f
	}, true)
}

// Trying to overwrite a directory fails
func TestLocalOverwriteDir(t *testing.T) {
	tmp, rm := TempDir()
	defer rm()
	f, r := someTestFile(tmp)
	err := os.Mkdir(f.Path, 777)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chtimes(f.Path, time.Now(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if local(f, r) == nil {
		t.Error("should have failed")
	}
}

func testLocal(t *testing.T, init func(File, io.ReadCloser) File, overwrite bool) {
	tmp, rm := TempDir()
	of, r := someTestFile(tmp)
	f := init(of, r)
	err := local(f, r)
	if err != nil {
		t.Error(err)
	}
	if overwrite {
		of = f
	}
	checkFile(t, of)
	filepath.Walk(tmp, func(path string, info os.FileInfo, err error) error {
		t.Log(path, info.ModTime(), err)
		return nil
	})
	rm()
}

func someTestFile(tmp string) (File, io.ReadCloser) {
	f := File{Mtime: time.Now(), Path: tmp + "/a"}
	return f, newFakeCloser("test")
}

func checkFile(t *testing.T, f File) {
	st, err := os.Stat(f.Path)

	f.Mtime = removeSubSecond(f.Mtime)

	if err != nil && os.IsNotExist(err) {
		t.Error("File does not exist:", f.Path)
	} else {
		mtime := removeSubSecond(st.ModTime())
		if !(mtime.Equal(f.Mtime)) {
			t.Errorf("Not overwritten")
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

func testTmp(t *testing.T) (name string, stop func()) {
	name, err := ioutil.TempDir("", "websync")
	if err != nil {
		t.Fatal(err)
	}
	name, err = filepath.EvalSymlinks(name)
	if err != nil {
		t.Fatal(err)
	}
	return name, func() {
		if err = os.RemoveAll(name); err != nil {
			t.Fatal(err)
		}
	}
}
