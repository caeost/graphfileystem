package graphfileystem

import (
	"fmt"
	"io"
	"testing"
)

func TestReadFiles(t *testing.T) {
	input := map[string]io.Reader{
		"abc":  NewMockFile("abc"),
		"abb":  NewMockFile("a"),
		"aaa":  NewMockFile("aaa"),
		"aaaa": NewMockFile("aaaaa"),
		"ooo":  NewMockFile("abc"),
	}

	gfs := New(input)

	if len(gfs.lookup) != 5 {
		t.Error("Wrong number of files, expected", 5, "got", len(gfs.lookup))
	}
}

func TestDeleteFile(t *testing.T) {
	input := map[string]io.Reader{
		"abc": NewMockFile("abc"),
		"abb": NewMockFile("a"),
	}

	gfs := NewStrict(input)

	if len(gfs.lookup) != 2 {
		t.Error("Wrong number of files, expected", 2, "got", len(gfs.lookup))
	}

	gfs.Delete("abc")

	if len(gfs.lookup) != 1 {
		t.Error("Wrong number of files, expected", 1, "got", len(gfs.lookup))
	}

	ok := <-gfs.Cleaned

	if !ok {
		t.Error("Error during cleanup")
	}

	if gfs.root.refs != 1 || len(gfs.root.children) != 0 || string(gfs.root.value) != "a" {
		t.Error("Cleanup failed")
		printNodes(gfs)
	}
}

// Helper functions and definitions
func printNodes(gfs impl) {
	fmt.Println("nodes")
	l := []*node{gfs.root}
	for len(l) > 0 {
		x := l[0]
		l = l[1:]

		for _, n := range x.children {
			l = append(l, n)
		}

		fmt.Println(x)
	}
}

func printLookups(gfs impl) {
	fmt.Println("lookups")
	for k, v := range gfs.lookup {
		fmt.Println(k, string(*v.path), v.length)
	}
}

func printFiles(gfs impl) {
	files := gfs.List()
	for k, v := range files {
		fmt.Println(k, string(v))
	}
}

func NewMockFile(contents string) *mockfile {
	return &mockfile{
		Contents: []byte(contents),
		Length:   len(contents),
		Cursor:   0,
	}
}

type mockfile struct {
	Contents []byte
	Length   int
	Cursor   int
}

func (m *mockfile) Read(p []byte) (n int, err error) {
	l := len(p)

	end := m.Cursor + l

	if end > m.Length {
		end = m.Length
	}

	n = copy(p, m.Contents[m.Cursor:end])

	if end == m.Length {
		err = io.EOF
	} else {
		err = nil
	}

	m.Cursor += l

	return
}
