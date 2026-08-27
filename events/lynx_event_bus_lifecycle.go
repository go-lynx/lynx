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

		if b.dispatcher != nil {
			return b.dispatcher.Close()
		}
		return nil
	}
	return nil
}
