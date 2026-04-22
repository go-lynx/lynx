package events

import (
	"context"

	kelindarEvent "github.com/kelindar/event"
)

// dispatchCatchAll calls all catch-all subscribers registered via Subscribe/SubscribeWithFilter.
func (b *LynxEventBus) dispatchCatchAll(ev LynxEvent) {
	b.catchAllSubs.Range(func(_, v any) bool {
		if h, ok := v.(func(LynxEvent)); ok {
			h(ev)
		}
		return true
	})
}

// wrapHandler wraps user handler with retry & panic recovery.
func (b *LynxEventBus) wrapHandler(handler func(LynxEvent)) func(LynxEvent) {
	return func(ev LynxEvent) {
		b.handleWithRetry(ev, handler, 1)
	}
}

// Subscribe subscribes to ALL events on this bus.
// We maintain our own catch-all list because kelindar/event routes by ev.Type(),
// so using kelindarEvent.Subscribe would only match events whose Type()==0.
func (b *LynxEventBus) Subscribe(handler func(LynxEvent)) context.CancelFunc {
	if b.isClosed.Load() {
		return func() {}
	}
	wrapped := b.wrapHandler(handler)
	id := b.catchAllSeq.Add(1)
	b.catchAllSubs.Store(id, wrapped)
	b.subscriberCount.Add(1)
	return func() {
		b.catchAllSubs.Delete(id)
		b.subscriberCount.Add(-1)
	}
}

// SubscribeTo subscribes to a specific event type on this bus.
func (b *LynxEventBus) SubscribeTo(eventType EventType, handler func(LynxEvent)) context.CancelFunc {
	if b.isClosed.Load() {
		return func() {}
	}
	wrapped := b.wrapHandler(handler)
	cancel := kelindarEvent.SubscribeTo(b.dispatcher, uint32(eventType), wrapped)
	b.subscriberCount.Add(1)
	b.mu.Lock()
	b.typeSubs[eventType]++
	b.mu.Unlock()
	return func() {
		cancel()
		b.subscriberCount.Add(-1)
		b.mu.Lock()
		if b.typeSubs[eventType] > 0 {
			b.typeSubs[eventType]--
		}
		b.mu.Unlock()
	}
}

// SubscribeWithFilter subscribes with a predicate filter (catch-all with guard).
func (b *LynxEventBus) SubscribeWithFilter(filter func(LynxEvent) bool, handler func(LynxEvent)) context.CancelFunc {
	if b.isClosed.Load() {
		return func() {}
	}
	wrapped := b.wrapHandler(func(ev LynxEvent) {
		if filter == nil || filter(ev) {
			handler(ev)
		}
	})
	id := b.catchAllSeq.Add(1)
	b.catchAllSubs.Store(id, wrapped)
	b.subscriberCount.Add(1)
	return func() {
		b.catchAllSubs.Delete(id)
		b.subscriberCount.Add(-1)
	}
}

// SubscribeToWithFilter subscribes to a specific event type with a predicate.
func (b *LynxEventBus) SubscribeToWithFilter(eventType EventType, filter func(LynxEvent) bool, handler func(LynxEvent)) context.CancelFunc {
	if b.isClosed.Load() {
		return func() {}
	}
	wrapped := b.wrapHandler(func(ev LynxEvent) {
		if filter == nil || filter(ev) {
			handler(ev)
		}
	})
	cancel := kelindarEvent.SubscribeTo(b.dispatcher, uint32(eventType), wrapped)
	b.subscriberCount.Add(1)
	b.mu.Lock()
	b.typeSubs[eventType]++
	b.mu.Unlock()
	return func() {
		cancel()
		b.subscriberCount.Add(-1)
		b.mu.Lock()
		if b.typeSubs[eventType] > 0 {
			b.typeSubs[eventType]--
		}
		b.mu.Unlock()
	}
}
