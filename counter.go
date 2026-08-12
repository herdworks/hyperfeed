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
	defer c.Lock.Unlock()
	c.Count += value
}

// json marshal method
func (c *Counter) MarshalJSON() ([]byte, error) {
	c.Lock.RLock()
	defer c.Lock.RUnlock()
	return json.Marshal(c.Count)
}
