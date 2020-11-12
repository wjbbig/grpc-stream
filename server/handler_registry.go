package server

import (
	"github.com/pkg/errors"
	"log"
	"sync"
)

// HandlerRegistry 用于管理不同client对应的handler
type HandlerRegistry struct {
	mutex sync.Mutex
	handlers map[string]*Handler
}

func newHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: map[string]*Handler{},
	}
}

func (hr *HandlerRegistry) Register(h *Handler) error {
	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	if hr.handlers[h.clientName] != nil {
		log.Printf("duplicate registered handler(key: %s) return error\n", h.clientName)
		return errors.Errorf("duplicate client: %s", h.clientName)
	}

	hr.handlers[h.clientName] = h

	log.Printf("registered handler complete for client %s\n", h.clientName)
	return nil
}

func (hr *HandlerRegistry) Deregister(name string) error {
	log.Printf("deregister handler: %s\n", name)

	hr.mutex.Lock()
	//handler := hr.handlers[name]
	delete(hr.handlers, name)
	hr.mutex.Unlock()

	return nil
}

// Handler 通过clientName获取指定的handler
func (r *HandlerRegistry) Handler(cName string) *Handler {
	r.mutex.Lock()
	h := r.handlers[cName]
	r.mutex.Unlock()
	return h
}