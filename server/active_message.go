package server

import "sync"

type ActiveMessages struct {
	mutex sync.Mutex
	ids map[string]struct{}
}

func NewActiveMessages() *ActiveMessages {
	return &ActiveMessages{
		ids:   map[string]struct{}{},
	}
}

func (a *ActiveMessages) Add(msgId string) bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if _, ok := a.ids[msgId]; !ok {
		a.ids[msgId] = struct{}{}
		return true
	}

	return false
}

func (a *ActiveMessages) Remove(msgId string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	delete(a.ids, msgId)
}
