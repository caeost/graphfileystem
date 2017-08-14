package graphfileystem

import (
	"io"
	"strconv"
)

func New(input map[string]io.Reader) impl {
	i := impl{
		root:   &node{[]byte{}, 0, map[byte]*node{}},
		lookup: map[string]*lookerup{},
		strict: false,
	}

	for name, file := range input {
		i.Insert(name, file) // pointer deref here?
	}

	return i
}

// Design:
// Read all the files within file system, storing contents, name, and metadata
// Iterate over all files contents character by character
//   adding child nodes to the previous character when a file has one
//   recording all the nodes the the child walks through (byte offset? # of chars is 256 per so should fit)
//   this means rather then storing each character we store based off of how many options there are at that point
//   But where is the actual savings cause we need to use a byte to store each offset potentially?
//   pigeon hole principle says that compression in each case is not theoretically possible
//
//   Store values within nodes while all children have the same next state
//
//   produce all the node outputs key to the graph paths
//   provide way to go backwards and pass over the path to produce the output

// main interface
type GraphFilesystem interface {
	Insert(name string, file io.Reader)
	Copy(target, name string) (ok bool)
	Delete(target string) (ok bool)
	Get(target string) (contents []byte, ok bool)
	Search(pcontents []byte) map[string][]byte
	List() map[string][]byte
}

type impl struct {
	root    *node
	lookup  map[string]*lookerup
	strict  bool
	Cleaned chan bool
}

func (i *impl) Insert(name string, file io.Reader) {
	// inserting same named file means ovewriting so delete old version
	if _, ok := i.lookup[name]; ok {
		i.Delete(name)
	}

	current := i.root
	cursor := 0

	path := []byte{}

	/*
		Okay so what is happening here?
		We read the file byte by byte and start building up our graph (really just a tree right now)
		First we check if the node is set, if it is not set that means we can freely add to it since no lookup is using it yet
			(means its under construction by the current insert)
		If it is not set then we add our value onto the end of it and move on with our lives
		If it is set then we have to do more work
			We want to check to see whether the byte is found in either the value or the children of the current node
				If it is in the value then this node still seems good, increment the cursor so we can check the next position against the next byte read
				If it is in the children switch the node to the child, and setup to start comparing against that one
				If it is in neither:
					Is it because the node's value isn't long enough?
						-> Create a new node as child to the current node
						-> save your byte into that child
						-> set the node equal to the child
						-> This node is not set so from here on out everything will just be added to it. Divergence! Scary!
					Is it because the next byte in the node's value is different?
						-> Split the node at this cursor position
						-> make the remainder a new node as a child of the current node
						-> find and update all the paths using this node
						-> make another child containing the current byte value
						-> use the current byte's node as your node
		"ref" the node
	*/

	splitNode := func(n *node, at int, path []byte) {
		expected := n.value[at]
		existingC := n.children

		// create two nodes now, one for old case one for new
		// reset n
		n.children = map[byte]*node{}
		// copy all children and references from node to this child
		n.children[expected] = &node{n.value[at:], n.refs, existingC}
		// set value of parent just to the common beginning
		n.value = n.value[:at]

		// now need to add this fork to the lookup for the old one(s)
		users := i.partialFind(path)
		for name, u := range users {
			lookup := i.lookup[name]
			lookup.path = insertPath(u, len(path), expected)
		}
	}

	length := 0
	p := make([]byte, 1)                                                                  // read one byte at a time y'all, this gets repeatedly overwritten so we can't use it the node creations below
	for nread, err := file.Read(p); nread != 0 || err == nil; nread, err = file.Read(p) { // complicated error EOF condition here: https://golang.org/pkg/io/#Reader
		if nread > 0 { // we have something to process!
			length++
			b := p[0] // more convenient to work with

			// first file in will only hit this state, it will write all of it into root
			if current.refs == 0 { // means that no finished file depends on the value of this node yet
				current.value = append(current.value, b)
				cursor++
			} else if cursor < len(current.value) && b == current.value[cursor] { // It is in the value
				cursor++ // we can continue adding to the same node... for now
			} else if current.children[b] != nil { // It is in the children
				cursor = 1
				current.refs++
				path = append(path, b)
				current = current.children[b]
			} else { // It is in neither
				if cursor < len(current.value) { // It is because the next byte in the node's value is different
					splitNode(current, cursor, path)
				}
				// If the node's value isn't long enough we just have to add another child

				// add the new child to the set of children
				current.children[b] = &node{[]byte{b}, 0, map[byte]*node{}}

				cursor = 0
				current.refs++
				path = append(path, b)
				// at this point we start writing an entirely other branch to the tree
				// would be nice to be able to merge back to what was common later, hard though!
				current = current.children[b]
			}
		}
	}

	if cursor < len(current.value) { // current node is actually longer then we need
		splitNode(current, cursor, path)
	}

	current.refs++

	i.lookup[name] = &lookerup{&path, length}
}

func (i impl) partialFind(path []byte) map[string]*[]byte {
	ret := map[string]*[]byte{}
	for name, lookup := range i.lookup {
		pat := lookup.path
		ok := true // clean this up later ugh
		l := len(*pat)
		for k, v := range path {
			p := *pat
			if k >= l || p[k] != v {
				ok = false
				break
			}
		}

		if ok {
			ret[name] = pat
		}
	}

	return ret
}

func insertPath(path *[]byte, at int, value byte) *[]byte {
	p := append(*path, 0)
	copy(p[at+1:], p[at:])
	p[at] = value

	return &p
}

// O(1)
func (i *impl) Copy(target, name string) bool {
	if target == name { // cannot copy into same name
		return false
	}

	lookup, ok := i.lookup[target]

	if !ok {
		return false
	}

	i.lookup[name] = &lookerup{lookup.path, lookup.length} // must be some way to copy structs?

	n := i.root
	for _, b := range *lookup.path {
		n.refs++
		n = n.children[b]
	}

	return true
}

// O(1)
func (i *impl) Delete(target string) bool {
	lookup, ok := i.lookup[target]

	if !ok {
		return false
	}

	delete(i.lookup, target)
	// we should probably clean up the node state, maybe in a goroutine
	go i.cleanup(*lookup.path) // might be some concurrency issues around cleaning up, mutex in cleanup? https://blog.golang.org/go-maps-in-action#TOC_6.

	return true
}

func (i *impl) cleanup(path []byte) {
	if i.strict {
		defer func() {
			i.Cleaned <- true
		}()
	}

	var parent *node = nil
	n := i.root
	var v byte
	for _, b := range path {
		v = b
		n.refs--
		parent = n
		n = n.children[b]
	}

	if n.refs == 1 && parent != nil {
		delete(parent.children, v)

		if len(parent.children) == 1 { // maybe heal split
			var b byte
			for k, _ := range parent.children { // this is kinda dumb
				b = k
			}

			child := parent.children[b]
			if parent.refs == child.refs { // heal split
				parent.children = child.children
				parent.value = append(parent.value, child.value...)
			}
		}
		// if we go to a graph and not a trie then we can't return here, we have to decrement refs all the way down
	}
}

// Note: may have to use pointer here for large situations anyway depending on what the obj looks like
// O(something)
func (i impl) Get(target string) ([]byte, bool) {
	lookup, ok := i.lookup[target]

	if !ok {
		return nil, false
	}

	node := i.root

	cursor := 0
	result := make([]byte, lookup.length)

	copy(result, node.value)
	cursor += len(node.value)

	for _, b := range *lookup.path {
		node = node.children[b]
		copy(result[cursor:], node.value)
		cursor += len(node.value)
	}

	return result, true
}

func (i impl) Search(pcontents []byte) map[string][]byte {
	// extract node traversal from Insert and put it in helper function
	// run the helper function on pcontents (partial contents) but without the deleting on existance / or saving to lookup
	// get path and compare to other paths (partialFind) to find all files starting with pcontents
	return map[string][]byte{}
}

func (i impl) List() map[string][]byte {
	res := map[string][]byte{}

	for name, _ := range i.lookup {
		v, ok := i.Get(name)

		if !ok {
			panic("Inconsistent file system!")
		}

		res[name] = v
	}

	return res
}

func NewStrict(input map[string]io.Reader) impl { // todo: avoid duplication, put underneath New()
	i := impl{
		root:    &node{[]byte{}, 0, map[byte]*node{}},
		lookup:  map[string]*lookerup{},
		strict:  true,
		Cleaned: make(chan bool),
	}

	for name, file := range input {
		i.Insert(name, file) // pointer deref here?
	}

	return i
}

type lookerup struct {
	path   *[]byte
	length int
}

type node struct {
	value []byte // technically don't need the first byte of each value since its in the parent but don't want to think of a compact version right now (interface{}{}?)
	// could maybe even be removed by logic inside of Insert keeping track of what is not "reffed"
	refs     int // "ref" means that the node has a complete definition in it / its children. aka adding another element to it could make an invalid previous file
	children map[byte]*node
}

func (m node) String() string { // implement Stringer for debugging
	names := []byte{}
	for c, _ := range m.children {
		names = append(names, c)
	}
	return "[" + string(m.value) + ", " + strconv.Itoa(m.refs) + ", [" + joinBytes(names) + "]]"
}

// type Reader interface {
//         Read(p []byte) (n int, err error)
// }
// Read 1 byte at a time into the array (might be more efficient to read more and do logic on our side to process down, or wrap Reader implementors)
type files map[string]io.Reader

func joinBytes(bytes []byte) string {
	s := ""
	for _, v := range bytes {
		s += string(v) + ","
	}

	l := len(s)
	if l > 1 {
		return s[:l-1]
	} else {
		return ""
	}
}
