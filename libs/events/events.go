/* Pubsub library. */
package events

import (
	"sync"
)

// Data passing from pub to sub.
type EventData interface {
}

type EventSwitch interface {
	// TODO(namdoh): Adds interface for start/stop/etc. of event bus.

	AddListenerForEvent(listenerID, event string, cb EventCallback)
	RemoveListenerForEvent(event string, listenerID string)
	RemoveListener(listenerID string)
	FireEvent(event string, data EventData)
}
