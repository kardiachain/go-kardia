/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package types

// EventBus is a common bus for all events going through the system. All calls
// are proxied to underlying pubsub server. All events must be published using
// EventBus to ensure correct data types.
type EventBus struct {
	// TODO(namdoh): Adds interface for start/stop/etc. of event bus.
	// TODO(namdoh): Adds field for kia/handler
}

func (b *EventBus) Publish(eventType string, eventData KaiEventData) error {
	// TODO(namdoh): Implement publishment via kia/handler.
	return nil
}

//--- EventDataRoundState events

func (b *EventBus) PublishEventNewRoundStep(event EventDataRoundState) error {
	return b.Publish(EventNewRoundStep, event)
}

func (b *EventBus) PublishEventNewRound(event EventDataRoundState) error {
	return b.Publish(EventNewRound, event)
}

func (b *EventBus) PublishEventPolka(event EventDataRoundState) error {
	return b.Publish(EventPolka, event)
}

func (b *EventBus) PublishEventCompleteProposal(event EventDataRoundState) error {
	return b.Publish(EventCompleteProposal, event)
}

func (b *EventBus) PublishEventUnlock(event EventDataRoundState) error {
	return b.Publish(EventUnlock, event)
}

func (b *EventBus) PublishEventRelock(event EventDataRoundState) error {
	return b.Publish(EventRelock, event)
}

func (b *EventBus) PublishEventLock(event EventDataRoundState) error {
	return b.Publish(EventLock, event)
}

func (b *EventBus) PublishEventVote(event EventDataVote) error {
	return b.Publish(EventVote, event)
}
