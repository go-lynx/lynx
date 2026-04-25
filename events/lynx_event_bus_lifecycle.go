package events

import (
	"context"
	"runtime"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// Close closes the event bus.
func (b *LynxEventBus) Close() error {
	if b.isClosed.CompareAndSwap(false, true) {
		b.enqueueMu.Lock()
		close(b.done)
		b.enqueueMu.Unlock()

		cfg, _, _, workerPool, _ := b.runtimeSnapshot()
		closeTimeout := cfg.CloseTimeout
		if closeTimeout <= 0 {
			closeTimeout = 30 * time.Second
		}

		done := make(chan struct{}, 1)
		ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
		defer cancel()

		goroutinesBefore := runtime.NumGoroutine()

		go func() {
			defer func() {
				select {
				case done <- struct{}{}:
				default:
				}
			}()
			b.wg.Wait()
		}()

		select {
		case <-done:
			if b.logger != nil {
				log.NewHelper(b.logger).Infof("event bus closed successfully, all goroutines finished")
			}
		case <-ctx.Done():
			goroutinesAfter := runtime.NumGoroutine()
			leakedGoroutines := goroutinesAfter - goroutinesBefore
			if b.logger != nil {
				log.NewHelper(b.logger).Warnf(
					"event bus close timeout after %v: %d goroutines may still be running (before: %d, after: %d), forcing cleanup",
					closeTimeout, leakedGoroutines, goroutinesBefore, goroutinesAfter)
			}
			select {
			case <-done:
			default:
			}
		}

		if workerPool != nil {
			if err := workerPool.ReleaseTimeout(closeTimeout); err != nil && b.logger != nil {
				log.NewHelper(b.logger).Warnf("worker pool release timeout: %v", err)
			}
		}

		b.processedEvents.Range(func(key, value any) bool {
			b.processedEvents.Delete(key)
			return true
		})

		if b.dispatcher != nil {
			return b.dispatcher.Close()
		}
		return nil
	}
	return nil
}

// cleanupProcessedEvents periodically cleans up old entries from processed events map.
func (b *LynxEventBus) cleanupProcessedEvents() {
	defer b.wg.Done()

	cleanupInterval := b.dedupWindow / 2
	if cleanupInterval < 30*time.Second {
		cleanupInterval = 30 * time.Second
	}
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	maxCleanupCount := 0

	for {
		select {
		case <-b.done:
			now := time.Now()
			cleaned := 0
			b.processedEvents.Range(func(key, value any) bool {
				if lastTime, ok := value.(time.Time); ok && now.Sub(lastTime) > b.dedupWindow {
					b.processedEvents.Delete(key)
					cleaned++
				}
				return true
			})
			if cleaned > 0 && b.logger != nil {
				log.NewHelper(b.logger).Debugf("final cleanup of processed events: removed %d entries", cleaned)
			}
			return
		case <-ticker.C:
			now := time.Now()
			cleaned := 0
			entryCount := 0
			b.processedEvents.Range(func(key, value any) bool {
				entryCount++
				return true
			})

			if entryCount > 0 {
				b.processedEvents.Range(func(key, value any) bool {
					if lastTime, ok := value.(time.Time); ok && now.Sub(lastTime) > b.dedupWindow {
						b.processedEvents.Delete(key)
						cleaned++
					}
					return true
				})

				if entryCount > maxCleanupCount {
					maxCleanupCount = entryCount
				}

				if cleaned > 1000 || (entryCount > 10000 && cleaned > 0) {
					if b.logger != nil {
						log.NewHelper(b.logger).Warnf(
							"processed events map cleanup: removed %d entries, remaining %d (max seen: %d)",
							cleaned, entryCount-cleaned, maxCleanupCount)
					}
				}
			}
		}
	}
}
