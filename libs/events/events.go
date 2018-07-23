/* Pubsub library. */
package events

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

func NewEventSwitch() EventSwitch {
	// TODO(namdoh): Implement.
	//evsw := &eventSwitch{
	//	eventCells: make(map[string]*eventCell),
	//	listeners:  make(map[string]*eventListener),
	//}
	//evsw.BaseService = *cmn.NewBaseService(nil, "EventSwitch", evsw)
	//return evsw

	panic("Missing implementation.")
	return nil
}

// -------- EVENT CALLBACK ---------
type EventCallback func(data EventData)
