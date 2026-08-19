package http

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
)

// TestLikeComment_ConcurrentDeterministic valida concorrência segura sem race conditions
// usando testing/synctest (Go 1.26). O tempo é virtualizado em bolha isolada: N devices
// distintos curtem o mesmo comentário em paralelo. Zero time.Sleep — sincronização 100%
// determinística via synctest.Test, eliminando testes intermitentes (flaky).
func TestLikeComment_ConcurrentDeterministic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockBeerUsecase := new(MockBeerUsecase)
		mockBeerUsecase.On("LikeComment", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		controller := NewBeerController(mockBeerUsecase, slog.Default(), nil, nil)

		const beerID, commentID = "1", "1"
		const workers = 16

		var wg sync.WaitGroup
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(device int) {
				defer wg.Done()
				req := httptest.NewRequest(http.MethodPost, "/beers/1/comments/1/like", nil)
				req.Header.Set("X-Device-ID", "device-"+itoa(device))
				rr := httptest.NewRecorder()
				req.SetPathValue("id", beerID)
				req.SetPathValue("commentId", commentID)
				controller.LikeComment(rr, req)
			}(i)
		}
		wg.Wait()
	})
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
