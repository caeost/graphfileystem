package graphfileystem

import "io"

func New(input map[string]*io.Reader) impl {
	i := impl{
		root:   &node{[]byte{}, false, map[byte]*node{}},
		lookup: map[string]*[]byte{},
	}

	for name, file := range input {
		i.Insert(name, *file) // pointer deref here?
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
	Find(target string) (file io.Reader, ok bool)
	Delete(target string) (ok bool)
	Copy(target, name string) (ok bool)
	Insert(name string, file io.Reader)
}

type node struct {
	value []byte // technically don't need the first byte of each value since its in the parent but don't want to think of a compact version right now (interface{}{}?)
	// set is only necessary during building, could be removed after
	set      bool // "set" means that the node has a complete definition in it / its children. aka adding another element to it could make an invalid previous file
	children map[byte]*node
}

type impl struct {
	root   *node
	lookup map[string]*[]byte
}

func (i *impl) Insert(name string, file io.Reader) {
	// inserting same named file means ovewriting so delete old version
	if _, ok := i.lookup[name]; ok {
		i.Delete(name)
	}

	current := i.root
	cursor := 0

	pattern := []byte{}

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
						-> find and update all the patterns using this node
						-> make another child containing the current byte value
						-> use the current byte's node as your node
		"set" the node
	*/

	p := make([]byte, 1)                                        // read one byte at a time y'all, this gets repeatedly overwritten so we can't use it the node creations below
	for nread, err := file.Read(p); nread != 0 || err == nil; { // complicated error EOF condition here: https://golang.org/pkg/io/#Reader
		if nread > 0 { // we have something to process!
			b := p[0] // more convenient to work with

			// first file in will only hit this state, it will write all of it into root
			if !current.set { // means that no finished file depends on the value of this node yet
				current.value = append(current.value, b)
			} else if cursor < len(current.value) && b == current.value[cursor] { // It is in the value
				cursor++ // we can continue adding to the same node... for now
			} else if current.children[b] != nil { // It is in the children
				cursor = 1

				current = current.children[b]

				pattern = append(pattern, b)
			} else { // It is in neither
				if cursor >= len(current.value) { // Is it because the node's value isn't long enough?
					current.children[b] = &node{[]byte{b}, false, map[byte]*node{}}
					current = current.children[b]
					pattern = append(pattern, b)
					cursor = 0
				} else { // Is it because the next byte in the node's value is different?
					expected := current.value[cursor]
					existingC := current.children

					// create two nodes now, one for old case one for new
					// reset
					current.children = map[byte]*node{}
					// copy all children from node to this child
					current.children[expected] = &node{current.value[:cursor], true, existingC}

					// now need to add this fork to the lookup for the old one(s)
					users := i.partialFind(pattern)
					for _, u := range users {
						insertPath(u, len(pattern), expected)
					}

					current.children[b] = &node{[]byte{b}, false, map[byte]*node{}}

					cursor = 0
					current.set = true
					pattern = append(pattern, b)
					// at this point we start writing an entirely other branch to the tree
					// would be nice to be able to merge back to what was common later, hard though!
					current = current.children[b]
				}
			}
		}
	}

	current.set = true

	i.lookup[name] = &pattern
}

func (i impl) partialFind(pattern []byte) []*[]byte {
	ret := []*[]byte{}
	for _, pat := range i.lookup {
		ok := true // clean this up later ugh
		for k, v := range pattern {
			p := *pat
			if p[k] != v {
				ok = false
				break
			}
		}

		if ok {
			ret = append(ret, pat)
		}
	}

	return ret
}

func insertPath(path *[]byte, at int, value byte) {
	// do that blah blah
}

func (i *impl) Copy(target, name string) bool {
	return false
}

func (i *impl) Delete(target string) bool {
	return false
}

// Note: may have to use pointer here for large situations anyway depending on what the obj looks like
func (i impl) Find(target string) bool {
	return false
}

// type Reader interface {
//         Read(p []byte) (n int, err error)
// }
// Read 1 byte at a time into the array (might be more efficient to read more and do logic on our side to process down, or wrap Reader implementors)
type files map[string]*io.Reader
