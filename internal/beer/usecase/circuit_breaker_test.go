package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"beer-review-app/internal/beer/model"

	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/mock"
	"testing/synctest"
)

// newTestBreaker cria um circuit breaker com janelas curtas para teste
// determinístico (timeout em segundos virtuais, não reais).
func newTestBreaker() *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "test-beer-service",
		MaxRequests: 1,
		Interval:    2 * time.Second,
		Timeout:     1 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.Requests >= 3 && counts.TotalFailures >= 3
		},
	})
}

// TestCircuitBreakerTripsUnderFailure usa testing/synctest para avançar o
// tempo virtualmente e verificar a transição Closed -> Open sem time.Sleep real.
func TestCircuitBreakerTripsUnderFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cb := newTestBreaker()
		fail := func() (interface{}, error) { return nil, errors.New("db down") }

		// 3 falhas consecutivas disparam o circuito (ReadyToTrip).
		for i := 0; i < 3; i++ {
			if _, err := cb.Execute(fail); err == nil {
				t.Fatalf("expected failure on attempt %d", i)
			}
		}

		if cb.State() != gobreaker.StateOpen {
			t.Fatalf("expected circuit Open, got %v", cb.State())
		}

		// Após o Timeout virtual, transição para Half-Open.
		time.Sleep(2 * time.Second) // tempo virtualizado dentro da synctest bubble

		if _, err := cb.Execute(func() (interface{}, error) {
			return model.Beer{ID: "1"}, nil
		}); err != nil {
			t.Fatalf("expected success in half-open state: %v", err)
		}
	})
}

// TestBeerUsecaseConcurrentGetByID garante segurança sob concorrência real
// usando sync.WaitGroup (sem sleeps frágeis).
func TestBeerUsecaseConcurrentGetByID(t *testing.T) {
	repo := new(MockBeerRepository)
	repo.On("GetByID", mock.Anything, "1").Return(model.Beer{ID: "1", Name: "Beer1"}, nil)

	u := NewBeerUsecase(repo, nil)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, err := u.GetByID(context.Background(), "1"); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
}
