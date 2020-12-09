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

import (
	"context"
	"github.com/kardiachain/go-kardiamain/lib/log"

	kpubsub "github.com/kardiachain/go-kardiamain/lib/pubsub"
	"github.com/kardiachain/go-kardiamain/lib/service"
)

const defaultCapacity = 0

type Subscription interface {
	Out() <-chan kpubsub.Message
	Cancelled() <-chan struct{}
	Err() error
}

// EventBus is a common bus for all events going through the system. All calls
// are proxied to underlying pubsub server. All events must be published using
// EventBus to ensure correct data types.
type EventBus struct {
	service.BaseService
	pubsub *kpubsub.Server
	// TODO(namdoh): Adds interface for start/stop/etc. of event bus.
	// TODO(namdoh): Adds field for kia/handler
}

// NewEventBus returns a new event bus.
func NewEventBus() *EventBus {
	return NewEventBusWithBufferCapacity(defaultCapacity)
}

// NewEventBusWithBufferCapacity returns a new event bus with the given buffer capacity.
func NewEventBusWithBufferCapacity(cap int) *EventBus {
	// capacity could be exposed later if needed
	pubsub := kpubsub.NewServer(kpubsub.BufferCapacity(cap))
	b := &EventBus{pubsub: pubsub}
	b.BaseService = *service.NewBaseService(nil, "EventBus", b)
	return b
}

func (b *EventBus) SetLogger(l log.Logger) {
	b.BaseService.SetLogger(l)
	b.pubsub.SetLogger(l.New("module", "pubsub"))
}

func (b *EventBus) OnStart() error {
	return b.pubsub.Start()
}

func (b *EventBus) OnStop() {
	if err := b.pubsub.Stop(); err != nil {
		b.pubsub.Logger.Error("error trying to stop eventBus", "error", err)
	}
}

func (b *EventBus) NumClients() int {
	return b.pubsub.NumClients()
}

func (b *EventBus) NumClientSubscriptions(clientID string) int {
	return b.pubsub.NumClientSubscriptions(clientID)
}

func (b *EventBus) Subscribe(
	ctx context.Context,
	subscriber string,
	query kpubsub.Query,
	outCapacity ...int,
) (Subscription, error) {
	return b.pubsub.Subscribe(ctx, subscriber, query, outCapacity...)
}

// This method can be used for a local consensus explorer and synchronous
// testing. Do not use for for public facing / untrusted subscriptions!
func (b *EventBus) SubscribeUnbuffered(
	ctx context.Context,
	subscriber string,
	query kpubsub.Query,
) (Subscription, error) {
	return b.pubsub.SubscribeUnbuffered(ctx, subscriber, query)
}

func (b *EventBus) Unsubscribe(ctx context.Context, subscriber string, query kpubsub.Query) error {
	return b.pubsub.Unsubscribe(ctx, subscriber, query)
}

func (b *EventBus) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return b.pubsub.UnsubscribeAll(ctx, subscriber)
}

func (b *EventBus) Publish(eventType string, eventData KaiEventData) error {
	// no explicit deadline for publishing events
	ctx := context.Background()
	return b.pubsub.PublishWithEvents(ctx, eventData, map[string][]string{EventTypeKey: {eventType}})
}

// validateAndStringifyEvents takes a slice of event objects and creates a
// map of stringified events where each key is composed of the event
// type and each of the event's attributes keys in the form of
// "{event.Type}.{attribute.Key}" and the value is each attribute's value.
// func (b *EventBus) validateAndStringifyEvents(events []types.Event, logger log.Logger) map[string][]string {
// 	result := make(map[string][]string)
// 	for _, event := range events {
// 		if len(event.Type) == 0 {
// 			logger.Debug("Got an event with an empty type (skipping)", "event", event)
// 			continue
// 		}

// 		for _, attr := range event.Attributes {
// 			if len(attr.Key) == 0 {
// 				logger.Debug("Got an event attribute with an empty key(skipping)", "event", event)
// 				continue
// 			}

// 			compositeTag := fmt.Sprintf("%s.%s", event.Type, string(attr.Key))
// 			result[compositeTag] = append(result[compositeTag], string(attr.Value))
// 		}
// 	}

// 	return result
// }

//func (b *EventBus) validateAndStringifyEvents(events []types.Event, logger log.Logger) map[string][]string {
//	fmt.Println("not yet implement")
//	return nil
//}

//--- EventDataRoundState events
func (b *EventBus) PublishEventNewRoundStep(event EventDataRoundState) error {
	return b.Publish(EventNewRoundStep, event)
}

func (b *EventBus) PublishEventTimeoutPropose(data EventDataRoundState) error {
	return b.Publish(EventTimeoutPropose, data)
}

func (b *EventBus) PublishEventTimeoutWait(data EventDataRoundState) error {
	return b.Publish(EventTimeoutWait, data)
}

func (b *EventBus) PublishEventNewRound(data EventDataNewRound) error {
	return b.Publish(EventNewRound, data)
}

func (b *EventBus) PublishEventCompleteProposal(data EventDataCompleteProposal) error {
	return b.Publish(EventCompleteProposal, data)
}

func (b *EventBus) PublishEventValidBlock(event EventDataRoundState) error {
	return b.Publish(EventValidBlock, event)
}

func (b *EventBus) PublishEventPolka(event EventDataRoundState) error {
	return b.Publish(EventPolka, event)
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

func (b *EventBus) PublishEventNewBlock(data EventDataNewBlock) error {
	return b.Publish(EventNewBlock, data)
}

func (b *EventBus) PublishEventNewBlockHeader(data EventDataNewBlockHeader) error {
	return b.Publish(EventNewBlockHeader, data)
}
