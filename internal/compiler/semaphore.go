package compiler

type semaphore struct {
	x chan bool
}

func newSemaphore(v int) *semaphore {
	return &semaphore{
		x: make(chan bool, v),
	}
}
func (self *semaphore) Lock() {
	self.x <- false
}

func (self *semaphore) Unlock() {
	<-self.x
}
