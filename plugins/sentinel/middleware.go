package sentinel

import (
	"fmt"
	"net/http"
)

// CreateHTTPMiddleware creates HTTP middleware for Sentinel protection
func (s *PlugSentinel) CreateHTTPMiddleware(resourceExtractor func(interface{}) string) interface{} {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resource := resourceExtractor(r)
			if resource == "" {
				resource = r.URL.Path
			}

			entry, err := s.Entry(resource)
			if err != nil {
				// Request blocked by Sentinel
				http.Error(w, fmt.Sprintf("Request blocked: %v", err), http.StatusTooManyRequests)
				return
			}

			// Execute the next handler
			defer func() {
				if entry != nil {
					// Exit the entry (this would be done by the actual Sentinel entry)
					// For now, we'll just log it
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// CreateGRPCInterceptor creates gRPC interceptor for Sentinel protection
func (s *PlugSentinel) CreateGRPCInterceptor() interface{} {
	// Return a placeholder interceptor function
	// In a real implementation, this would return proper gRPC interceptor types
	return func(method string) error {
		entry, err := s.Entry(method)
		if err != nil {
			return fmt.Errorf("request blocked by Sentinel: %w", err)
		}

		// In real implementation, we would properly handle the entry lifecycle
		_ = entry

		return nil
	}
}