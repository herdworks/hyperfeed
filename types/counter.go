package hyperfeed

import (
	"encoding/json"
	"sync"
)

type Counter struct {
	Count int          `json:"count"`
	Lock  sync.RWMutex `json:"-"`
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

// json marshal method
func (c *Counter) MarshalJSON() ([]byte, error) {
	c.Lock.RLock()
	defer c.Lock.RUnlock()
	return json.Marshal(c.Count)
}
