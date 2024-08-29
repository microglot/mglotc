// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package idl

type PathState struct {
	// these keep track of the current "path" during conversion
	// Path is a highly specialized and compact "index" into a FileDescriptor, used
	// to associate optional SourceCodeInfo with specific elements of the FileDescriptor.
	path       []int32
	indexStack []int
}

func (p *PathState) PushFieldNumber(fieldNumber int32) {
	p.path = append(p.path, fieldNumber)
}

func (p *PathState) PopFieldNumber() {
	p.path = p.path[:len(p.path)-1]
}

func (p *PathState) PushIndex() {
	p.path = append(p.path, 0)
	p.indexStack = append(p.indexStack, len(p.path)-1)
}

func (p *PathState) PopIndex() {
	p.path = p.path[:len(p.path)-1]
	p.indexStack = p.indexStack[:len(p.indexStack)-1]
}

func (p *PathState) IncrementIndex() {
	if len(p.indexStack) > 0 {
		p.path[p.indexStack[len(p.indexStack)-1]] += 1
	}
}

func (p *PathState) CopyPath() []int32 {
	c := make([]int32, len(p.path))
	copy(c, p.path)
	return c
}
