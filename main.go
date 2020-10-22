package main

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"golang.org/x/time/rate"
)

const (
	botLimit    = 10
	botInterval = time.Minute
)

func main() {
	rest := NewRest(botInterval, botLimit)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Get("/count", rest.Count)
	router.Get("/touch/{user_id}", rest.Touch)

	http.ListenAndServe(":8000", router)
}

type Rest struct {
	interval time.Duration
	limit    int

	users map[string]*rate.Limiter
	mu    sync.Mutex
}

func NewRest(interval time.Duration, limit int) *Rest {
	return &Rest{
		interval: interval,
		limit:    limit,

		users: make(map[string]*rate.Limiter),
	}
}

// Count returns bots count. Time complexity is O(n), where n is total users count.
func (s *Rest) Count(w http.ResponseWriter, r *http.Request) {
	var count int
	s.mu.Lock()
	for _, limiter := range s.users {
		limiterCopy := *limiter   // get a copy to avoid one more countable request below
		if !limiterCopy.Allow() { // thread-safe
			count++
		}
	}
	s.mu.Unlock()
	render.PlainText(w, r, strconv.Itoa(count))
}

// Touch increases smoothed request counter per a user. Time complexity is O(1).
func (s *Rest) Touch(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	limiter := s.getUserLimiter(userID)
	limiter.Reserve() // thread-safe
	render.PlainText(w, r, "OK")
}

func (s *Rest) getUserLimiter(userID string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	limiter, found := s.users[userID]
	if !found {
		limit := s.limit + 1
		limiter = rate.NewLimiter(rate.Every(s.interval/time.Duration(limit)), limit)
		s.users[userID] = limiter
	}
	return limiter
}
