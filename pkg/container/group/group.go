package group

import (
	"errors"
	"sync"
)

// Group is a lazy load container.
type Group struct {
	new  func() interface{}
	objs sync.Map
	sync.RWMutex
}

// NewGroup news a group container.
func NewGroup(new func() interface{}) (group *Group, err error) {
	if new == nil {
		return nil, errors.New("container.group: can't assign a nil to the new function")
	}
	group = &Group{
		new: new,
	}
	return group, nil
}

// Get gets the object by the given key.
func (g *Group) Get(key string) interface{} {
	g.RLock()
	n := g.new
	g.RUnlock()
	obj, ok := g.objs.Load(key)
	if !ok {
		obj = n()
		g.objs.Store(key, obj)
	}
	return obj
}

// Reset resets the new function and deletes all existing objects.
func (g *Group) Reset(new func() interface{}) (err error) {
	if new == nil {
		return errors.New("container.group: can't assign a nil to the new function")
	}
	g.Lock()
	g.new = new
	g.Unlock()
	g.Clear()
	return
}

// Clear deletes all objects.
func (g *Group) Clear() {
	g.objs.Range(func(key, value interface{}) bool {
		g.objs.Delete(key)
		return true
	})
}
