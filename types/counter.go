package hyperfeed

import (
	"sync"
)

type Counter struct {
	Count int
	Lock  sync.RWMutex
}

func (c *Counter) Increment(value int) {
	c.Lock.Lock()
	c.Count += value
	c.Lock.Unlock()
}

func (c *Counter) Reset() {
	c.Count = 0
	c.Lock = sync.RWMutex{}
}
