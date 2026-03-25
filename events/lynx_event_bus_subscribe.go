package events

import (
	"context"

	kelindarEvent "github.com/kelindar/event"
)

// wrapHandler wraps user handler with retry & panic recovery.
func (b *LynxEventBus) wrapHandler(handler func(LynxEvent)) func(LynxEvent) {
	return func(ev LynxEvent) {
		b.handleWithRetry(ev, handler, 1)
	}
}

// Subscribe subscribes to events on this bus.
func (b *LynxEventBus) Subscribe(handler func(LynxEvent)) context.CancelFunc {
	if b.isClosed.Load() {
		return func() {}
	}
	wrapped := b.wrapHandler(handler)
	cancel := kelindarEvent.Subscribe(b.dispatcher, wrapped)
	b.subscriberCount.Add(1)
	return func() {
		cancel()
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

// SubscribeWithFilter subscribes with a predicate filter.
func (b *LynxEventBus) SubscribeWithFilter(filter func(LynxEvent) bool, handler func(LynxEvent)) context.CancelFunc {
	if b.isClosed.Load() {
		return func() {}
	}
	wrapped := b.wrapHandler(func(ev LynxEvent) {
		if filter == nil || filter(ev) {
			handler(ev)
		}
	})
	cancel := kelindarEvent.Subscribe(b.dispatcher, wrapped)
	b.subscriberCount.Add(1)
	return func() {
		cancel()
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
