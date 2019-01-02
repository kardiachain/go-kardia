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

package event_pool

import (
	"container/heap"
	"errors"
	"io"
	"os"
	"sort"

	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"
)

// nonceHeap is a heap.Interface implementation over 64bit unsigned integers for
// retrieving sorted dual's event from the possibly gapped future queue.
type nonceHeap []uint64

func (h nonceHeap) Len() int           { return len(h) }
func (h nonceHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h nonceHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *nonceHeap) Push(x interface{}) {
	*h = append(*h, x.(uint64))
}

func (h *nonceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// A nonce->dual's event hash map with a heap based index to allow
// iterating over the contents in a nonce-incrementing way.
type eventSortedMap struct {
	items map[uint64]*types.DualEvent // Hash map storing the dual's event data
	index *nonceHeap                  // Heap of nonces of all the stored dual's event (non-strict mode)
	cache types.DualEvents            // Cache of the dual's event already sorted
}

// Creates a new nonce-sorted dual's event map.
func newEventSortedMap() *eventSortedMap {
	return &eventSortedMap{
		items: make(map[uint64]*types.DualEvent),
		index: new(nonceHeap),
	}
}

// Retrieves the current dual's event associated with the given nonce.
func (m *eventSortedMap) Get(nonce uint64) *types.DualEvent {
	return m.items[nonce]
}

// Put inserts a new dual's event into the map, also updating the map's nonce
// index. If a dual's event already exists with the same nonce, it's overwritten.
func (m *eventSortedMap) Put(event *types.DualEvent) {
	nonce := event.Nonce
	if m.items[nonce] == nil {
		heap.Push(m.index, nonce)
	}
	m.items[nonce], m.cache = event, nil
}

// Removes all dual's events from the map with a nonce lower than the
// provided threshold. Every removed dual's event is returned for any post-removal
// maintenance.
func (m *eventSortedMap) Forward(threshold uint64) types.DualEvents {
	var removed types.DualEvents

	// Pop off heap items until the threshold is reached
	for m.index.Len() > 0 && (*m.index)[0] < threshold {
		nonce := heap.Pop(m.index).(uint64)
		removed = append(removed, m.items[nonce])
		delete(m.items, nonce)
	}
	// If we had a cached order, shift the front
	if m.cache != nil {
		m.cache = m.cache[len(removed):]
	}
	return removed
}

// Iterates over the list of dual's events and removes all of them for which
// the specified function evaluates to true.
func (m *eventSortedMap) Filter(filter func(*types.DualEvent) bool) types.DualEvents {
	var removed types.DualEvents

	// Collect all the dual's events to filter out
	for nonce, event := range m.items {
		if filter(event) {
			removed = append(removed, event)
			delete(m.items, nonce)
		}
	}
	// If dual's events were removed, the heap and cache are ruined
	if len(removed) > 0 {
		*m.index = make([]uint64, 0, len(m.items))
		for nonce := range m.items {
			*m.index = append(*m.index, nonce)
		}
		heap.Init(m.index)

		m.cache = nil
	}
	return removed
}

// Places a hard limit on the number of items, returning all dual's events
// exceeding that limit.
func (m *eventSortedMap) Cap(threshold int) types.DualEvents {
	// Short circuit if the number of items is under the limit
	if len(m.items) <= threshold {
		return nil
	}
	// Otherwise gather and drop the highest nonce'd dual's events
	var drops types.DualEvents

	sort.Sort(*m.index)
	for size := len(m.items); size > threshold; size-- {
		drops = append(drops, m.items[(*m.index)[size-1]])
		delete(m.items, (*m.index)[size-1])
	}
	*m.index = (*m.index)[:threshold]
	heap.Init(m.index)

	// If we had a cache, shift the back
	if m.cache != nil {
		m.cache = m.cache[:len(m.cache)-len(drops)]
	}
	return drops
}

// Deletes a dual's event from the maintained map, returning whether the
// event was found.
func (m *eventSortedMap) Remove(nonce uint64) bool {
	// Short circuit if no dual's events is present
	_, ok := m.items[nonce]
	if !ok {
		return false
	}
	// Otherwise delete the dual's event and fix the heap index
	for i := 0; i < m.index.Len(); i++ {
		if (*m.index)[i] == nonce {
			heap.Remove(m.index, i)
			break
		}
	}
	delete(m.items, nonce)
	m.cache = nil

	return true
}

// Retrieves a sequentially increasing list of dual's events starting at the
// provided nonce that is ready for processing. The returned dual's events will be
// removed from the list.
//
// Note, all dual's events with nonces lower than start will also be returned to
// prevent getting into and invalid state. This is not something that should ever
// happen but better to be self correcting than failing!
func (m *eventSortedMap) Ready(start uint64) types.DualEvents {
	// Short circuit if no dual's events are available
	if m.index.Len() == 0 || (*m.index)[0] > start {
		return nil
	}
	// Otherwise start accumulating incremental dual's events
	var ready types.DualEvents
	for next := (*m.index)[0]; m.index.Len() > 0 && (*m.index)[0] == next; next++ {
		ready = append(ready, m.items[next])
		delete(m.items, next)
		heap.Pop(m.index)
	}
	m.cache = nil

	return ready
}

// Len returns the length of the dual's event map.
func (m *eventSortedMap) Len() int {
	return len(m.items)
}

// Flatten creates a nonce-sorted slice of dual's events based on the loosely
// sorted internal representation. The result of the sorting is cached in case
// it's requested again before any modifications are made to the contents.
func (m *eventSortedMap) Flatten() types.DualEvents {
	// If the sorting was not cached yet, create and cache it
	if m.cache == nil {
		m.cache = make(types.DualEvents, 0, len(m.items))
		for _, tx := range m.items {
			m.cache = append(m.cache, tx)
		}
		sort.Sort(types.DualEventByNonce(m.cache))
	}
	// Copy the cache to prevent accidental modifications
	events := make(types.DualEvents, len(m.cache))
	copy(events, m.cache)
	return events
}

// A "list" of dual's event belong to this dual node.
type eventList struct {
	events *eventSortedMap // Heap indexed sorted hash map of the dual's events
}

// Creates a new dual's event list for maintaining nonce-indexable fast,
// gapped, sortable dual's event list.
func newEventList() *eventList {
	return &eventList{
		events: newEventSortedMap(),
	}
}

// Returns whether the dual's event specified has the same nonce as one
// already contained within the list.
func (l *eventList) Overlaps(event *types.DualEvent) bool {
	return l.events.Get(event.Nonce) != nil
}

// Tries to insert a new dual's event into the list, returning whether the
// dual's event was inserted.
func (l *eventList) Add(event *types.DualEvent) bool {
	// If there's an existing dual's event, abort
	if existing := l.events.Get(event.Nonce); existing != nil {
		return false
	}

	l.events.Put(event)
	return true
}

// Removes all dual's events from the list with a nonce lower than the
// provided threshold. Every removed dual's events is returned for any post-removal
// maintenance.
func (l *eventList) Forward(threshold uint64) types.DualEvents {
	return l.events.Forward(threshold)
}

// Places a hard limit on the number of items, returning all dual's events
// exceeding that limit.
func (l *eventList) Cap(threshold int) types.DualEvents {
	return l.events.Cap(threshold)
}

// Deletes a dual's event from the maintained list, returning whether the
// event was found.
func (l *eventList) Remove(event *types.DualEvent) bool {
	// Remove the dual's event from the set
	if removed := l.events.Remove(event.Nonce); !removed {
		return false
	}
	return true
}

// Retrieves a sequentially increasing list of dual's events starting at the
// provided nonce that is ready for processing. The returned dual's events will be
// removed from the list.
//
// Note, all dual's events with nonces lower than start will also be returned to
// prevent getting into and invalid state. This is not something that should ever
// happen but better to be self correcting than failing!
func (l *eventList) Ready(start uint64) types.DualEvents {
	return l.events.Ready(start)
}

// Len returns the length of the dual's event list.
func (l *eventList) Len() int {
	return l.events.Len()
}

// Empty returns whether the list of dual's events is empty or not.
func (l *eventList) Empty() bool {
	return l.Len() == 0
}

// Flatten creates a nonce-sorted slice of dual's events based on the loosely
// sorted internal representation. The result of the sorting is cached in case
// it's requested again before any modifications are made to the contents.
func (l *eventList) Flatten() types.DualEvents {
	return l.events.Flatten()
}

//==============================================================================================
// event_journal
//==============================================================================================
// errNoActiveJournal is returned if a dual's event is attempted to be inserted
// into the journal, but no such file is currently open.
var errNoActiveJournal = errors.New("no active journal")

// devNull is a WriteCloser that just discards anything written into it. Its
// goal is to allow the dual's event journal to write into a fake journal when
// loading dual's events on startup without printing warnings due to no file
// being readt for write.
type devNull struct{}

func (*devNull) Write(p []byte) (n int, err error) { return len(p), nil }
func (*devNull) Close() error                      { return nil }

// eventJournal is a rotating log of dual's events with the aim of storing locally
// created dual's events to allow non-executed ones to survive node restarts.
type eventJournal struct {
	logger log.Logger
	path   string         // Filesystem path to store the dual's events at
	writer io.WriteCloser // Output stream to write new dual's events into
}

// Creates a new dual's event journal to
func newEventJournal(logger log.Logger, path string) *eventJournal {
	return &eventJournal{
		logger: logger,
		path:   path,
	}
}

// Parses a dual's event journal dump from disk, loading its contents into
// the specified pool.
func (journal *eventJournal) load(add func([]*types.DualEvent) []error) error {
	// Skip the parsing if the journal file doens't exist at all
	if _, err := os.Stat(journal.path); os.IsNotExist(err) {
		return nil
	}
	// Open the journal for loading any past dual's events
	input, err := os.Open(journal.path)
	if err != nil {
		return err
	}
	defer input.Close()

	// Temporarily discard any journal additions (don't double add on load)
	journal.writer = new(devNull)
	defer func() { journal.writer = nil }()

	// Inject all dual's event from the journal into the pool
	stream := rlp.NewStream(input, 0)
	total, dropped := 0, 0

	// Create a method to load a limited batch of dual's events and bump the
	// appropriate progress counters. Then use this method to load all the
	// journalled dual's events in small-ish batches.
	loadBatch := func(events types.DualEvents) {
		for _, err := range add(events) {
			if err != nil {
				journal.logger.Debug("Failed to add journaled dual's events", "err", err)
				dropped++
			}
		}
	}
	var (
		failure error
		batch   types.DualEvents
	)
	for {
		// Parse the next dual's event and terminate on error
		event := new(types.DualEvent)
		if err = stream.Decode(event); err != nil {
			if err != io.EOF {
				failure = err
			}
			if batch.Len() > 0 {
				loadBatch(batch)
			}
			break
		}
		// New dual's event parsed, queue up for later, import if threshold is reached
		total++

		if batch = append(batch, event); batch.Len() > 1024 {
			loadBatch(batch)
			batch = batch[:0]
		}
	}
	journal.logger.Info("Loaded local dual's event journal", "dual's events", total, "dropped", dropped)

	return failure
}

// Adds the specified dual's event to the local disk journal.
func (journal *eventJournal) insert(event *types.DualEvent) error {
	if journal.writer == nil {
		return errNoActiveJournal
	}
	if err := rlp.Encode(journal.writer, event); err != nil {
		return err
	}
	return nil
}

// Regenerates the dual's event journal based on the current contents of
// the dual's event pool.
func (journal *eventJournal) rotate(events types.DualEvents) error {
	// Close the current journal (if any is open)
	if journal.writer != nil {
		if err := journal.writer.Close(); err != nil {
			return err
		}
		journal.writer = nil
	}
	// Generate a new journal with the contents of the current pool
	replacement, err := os.OpenFile(journal.path+".new", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err = rlp.Encode(replacement, event); err != nil {
			replacement.Close()
			return err
		}
	}
	journaled := len(events)
	replacement.Close()

	// Replace the live journal with the newly generated one
	if err = os.Rename(journal.path+".new", journal.path); err != nil {
		return err
	}
	sink, err := os.OpenFile(journal.path, os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		return err
	}
	journal.writer = sink
	journal.logger.Info("Regenerated local dual's event journal", "dual's event", journaled)

	return nil
}

// Flushes the dual's event journal contents to disk and closes the file.
func (journal *eventJournal) close() error {
	var err error

	if journal.writer != nil {
		err = journal.writer.Close()
		journal.writer = nil
	}
	return err
}
