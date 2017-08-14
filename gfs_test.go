package graphfileystem

import (
	"fmt"
	"io"
	"testing"
)

func TestReadFile(t *testing.T) {
	input := map[string]io.Reader{
		"abc": NewMockFile("abc"),
		"abb": NewMockFile("abb"),
		"aaa": NewMockFile("aaa"),
		"ooo": NewMockFile("abc"),
	}

	gfs := New(input)

	files := gfs.List()

	fmt.Println("paths")
	for k, v := range gfs.lookup {
		fmt.Println(k, string(*v))
	}

	fmt.Println("file names and contents")
	for k, v := range files {
		fmt.Println(k, string(v))
	}

	fmt.Println("nodes")
	l := []*node{gfs.root}
	for len(l) > 0 {
		x := l[0]
		l = l[1:]

		names := []byte{}
		for c, n := range x.children {
			names = append(names, c)
			l = append(l, n)
		}

		fmt.Println("[", string(x.value), ",", x.set, ",", string(names), "]")
	}

	if len(gfs.lookup) != 4 {
		t.Error("Wrong number of lookups, expected", 4, "got", len(gfs.lookup))
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
