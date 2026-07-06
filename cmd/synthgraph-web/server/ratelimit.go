package server

import (
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu              sync.Mutex
	visitors        map[string]*visitorEntry
	rate            int
	window          time.Duration
	cleanupInterval time.Duration
}

type visitorEntry struct {
	count       int
	windowStart time.Time
}

func newRateLimiter(rate int, window time.Duration) *rateLimiter {
	limiter := &rateLimiter{
		visitors:        make(map[string]*visitorEntry),
		rate:            rate,
		window:          window,
		cleanupInterval: window * 10,
	}
	go limiter.periodicCleanup()
	return limiter
}

func (limiter *rateLimiter) allow(visitorKey string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	entry, exists := limiter.visitors[visitorKey]
	now := time.Now()

	if !exists || now.Sub(entry.windowStart) > limiter.window {
		limiter.visitors[visitorKey] = &visitorEntry{
			count:       1,
			windowStart: now,
		}
		return true
	}

	if entry.count >= limiter.rate {
		return false
	}

	entry.count++
	return true
}

func (limiter *rateLimiter) periodicCleanup() {
	for {
		time.Sleep(limiter.cleanupInterval)
		limiter.mu.Lock()
		now := time.Now()
		for visitorKey, entry := range limiter.visitors {
			if now.Sub(entry.windowStart) > limiter.window {
				delete(limiter.visitors, visitorKey)
			}
		}
		limiter.mu.Unlock()
	}
}

var globalRateLimiter = newRateLimiter(60, 1*time.Minute)

func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		visitorKey := request.RemoteAddr
		if !globalRateLimiter.allow(visitorKey) {
			responseWriter.Header().Set("Retry-After", "60")
			writeError(responseWriter, http.StatusTooManyRequests, "rate limit exceeded, try again later")
			return
		}
		next.ServeHTTP(responseWriter, request)
	})
}
