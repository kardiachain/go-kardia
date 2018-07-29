/* Pubsub library. */
package events

import (
	"sync"
)

// Data passing from pub to sub.
type EventData interface {
}

//-------- EVENT SWITCH ---------
type EventSwitch interface {
	// TODO(namdoh): Adds interface for start/stop/etc. of event bus.

	AddListenerForEvent(listenerID, event string, cb EventCallback)
	RemoveListenerForEvent(event string, listenerID string)
	RemoveListener(listenerID string)
	FireEvent(event string, data EventData)
}

type eventSwitch struct {
	mtx        sync.RWMutex
	eventCells map[string]*eventCell
	listeners  map[string]*eventListener
}

func NewEventSwitch() EventSwitch {
	evsw := &eventSwitch{
		eventCells: make(map[string]*eventCell),
		listeners:  make(map[string]*eventListener),
	}
	return evsw
}

func (evsw *eventSwitch) AddListenerForEvent(listenerID, event string, cb EventCallback) {
	panic("events.AddListenerForEvent - Not yet implemented.")
	// Get/Create eventCell and listener
	//evsw.mtx.Lock()
	//eventCell := evsw.eventCells[event]
	//if eventCell == nil {
	//	eventCell = newEventCell()
	//	evsw.eventCells[event] = eventCell
	//}
	//listener := evsw.listeners[listenerID]
	//if listener == nil {
	//	listener = newEventListener(listenerID)
	//	evsw.listeners[listenerID] = listener
	//}
	//evsw.mtx.Unlock()
	//
	//// Add event and listener
	//eventCell.AddListener(listenerID, cb)
	//listener.AddEvent(event)
}

func (evsw *eventSwitch) RemoveListener(listenerID string) {
	panic("events.RemoveListener - Not yet implemented.")
	// Get and remove listener
	//evsw.mtx.RLock()
	//listener := evsw.listeners[listenerID]
	//evsw.mtx.RUnlock()
	//if listener == nil {
	//	return
	//}
	//
	//evsw.mtx.Lock()
	//delete(evsw.listeners, listenerID)
	//evsw.mtx.Unlock()
	//
	//// Remove callback for each event.
	//listener.SetRemoved()
	//for _, event := range listener.GetEvents() {
	//	evsw.RemoveListenerForEvent(event, listenerID)
	//}
}

func (evsw *eventSwitch) RemoveListenerForEvent(event string, listenerID string) {
	panic("events.RemoveListenerForEvent - Not yet implemented.")
	// Get eventCell
	//evsw.mtx.Lock()
	//eventCell := evsw.eventCells[event]
	//evsw.mtx.Unlock()
	//
	//if eventCell == nil {
	//	return
	//}
	//
	//// Remove listenerID from eventCell
	//numListeners := eventCell.RemoveListener(listenerID)
	//
	//// Maybe garbage collect eventCell.
	//if numListeners == 0 {
	//	// Lock again and double check.
	//	evsw.mtx.Lock()      // OUTER LOCK
	//	eventCell.mtx.Lock() // INNER LOCK
	//	if len(eventCell.listeners) == 0 {
	//		delete(evsw.eventCells, event)
	//	}
	//	eventCell.mtx.Unlock() // INNER LOCK
	//	evsw.mtx.Unlock()      // OUTER LOCK
	//}
}

func (evsw *eventSwitch) FireEvent(event string, data EventData) {
	panic("events.FireEvent - Not yet implemented.")
	// Get the eventCell
	//evsw.mtx.RLock()
	//eventCell := evsw.eventCells[event]
	//evsw.mtx.RUnlock()
	//
	//if eventCell == nil {
	//	return
	//}
	//
	//// Fire event for all listeners in eventCell
	//eventCell.FireEvent(data)
}

// eventCell handles keeping track of listener callbacks for a given event.
type eventCell struct {
	mtx       sync.RWMutex
	listeners map[string]EventCallback
}

type eventListener struct {
	id string

	mtx     sync.RWMutex
	removed bool
	events  []string
}

// -------- EVENT CALLBACK ---------
type EventCallback func(data EventData)
