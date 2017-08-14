package graphfileystem

import (
	"io"
	"testing"
)

func TestReadFile(t *testing.T) {
	input := map[string]*mockfile{
		"abc": NewMockFile("abc"),
		"abb": NewMockFile("abb"),
		"acc": NewMockFile("acc"),
		"aaa": NewMockFile("aaa"),
	}

	gfs := New(input)

	if len(gfs.lookup) < 4 {
		t.Error("not enough lookups expected", 4, "got", len(gfs.lookup))
	}
}

func NewMockFile(contents string) *mockfile {
	return &mockfile{
		Contents: []byte(contents),
		Cursor:   0,
	}
}

type mockfile struct {
	Contents []byte
	Cursor   int
}

func (m mockfile) Read(p []byte) (n int, err error) {
	l := len(p)

	n = copy(p, m.Contents[m.Cursor:(m.Cursor+l)])

	if (m.Cursor + l) >= len(m.Contents) {
		err = io.EOF
	} else {
		err = nil
	}

	m.Cursor += l

	return
}
