package events

import "sync"

type EventName string
type EventListener func(args ...interface{})

type Event struct {
	mu        sync.RWMutex
	listeners map[EventName]EventListener
}

func Emitter() *Event {
	return &Event{
		mu:        sync.RWMutex{},
		listeners: make(map[EventName]EventListener),
	}
}

func (e *Event) Emit(eventName EventName, args ...interface{}) {
	e.mu.RLock()
	listener, ok := e.listeners[eventName]
	e.mu.RUnlock()
	if ok && listener != nil {
		listener(args...)
	}
}

func (e *Event) On(eventName EventName, listener EventListener) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners[eventName] = listener
}

func (e *Event) Off(eventName EventName) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.listeners, eventName)
}

func (e *Event) Get(eventName EventName) EventListener {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.listeners[eventName]
}