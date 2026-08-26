package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"

	"github.com/stretchr/testify/mock"
)

// === Invariante de Poluição (Checklist do Supremo Testador #1) ===
// O payload de mutação NUNCA deve ditar campos protegidos do servidor
// (createdBy, createdAt, likes, id). O handler deve ignorá-los e injetar
// valores gerados pelo sistema (token de sessão / servidor).

func TestAddComment_IgnorePollutedProtectedFields(t *testing.T) {
	m := new(MockBeerUsecase)
	controller := NewBeerController(m, mockLogger, nil)

	var captured model.Comment
	m.On("AddComment", mock.Anything, "1", mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(2).(model.Comment)
		}).
		Return(nil).Once()

	// Cliente malicioso tenta forjar autor, timestamp, likes e id.
	payload := model.Comment{
		ID:        "forged-id",
		Text:      "Great beer!",
		Rating:    5,
		Likes:     999,
		LikedBy:   []string{"someone-else"},
		CreatedBy: "attacker",
		CreatedAt: "2000-01-01T00:00:00Z",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/beers/1/comments", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "real-user", ""))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	controller.AddComment(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	// O autor deve vir do token, não do payload.
	if captured.CreatedBy != "real-user" {
		t.Errorf("poluição: CreatedBy = %q, want %q (do token)", captured.CreatedBy, "real-user")
	}
	if captured.ID == "forged-id" {
		t.Errorf("poluição: ID não foi regerado pelo servidor (%q)", captured.ID)
	}
	if captured.Likes != 0 {
		t.Errorf("poluição: Likes = %d, want 0 (servidor)", captured.Likes)
	}
	if captured.CreatedAt == "2000-01-01T00:00:00Z" {
		t.Errorf("poluição: CreatedAt não foi sobrescrito pelo servidor")
	}
}

func TestCreateBeer_IgnorePollutedServerFields(t *testing.T) {
	m := new(MockBeerUsecase)
	controller := NewBeerController(m, mockLogger, nil)

	var captured *model.Beer
	m.On("Create", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(*model.Beer)
		}).
		Return(nil).Once()

	abv := 5.0
	payload := model.Beer{
		ID:          "forged",
		Name:        "Test IPA",
		Style:       "IPA",
		Alcohol:     &abv,
		Taste:       "Amargo",
		Aroma:       "Cítrico",
		Color:       "Âmbar",
		Body:        "Médio",
		Carbonation: "Alta",
		Finish:      "Seco",
		CreatedBy:   "attacker",
		CreatedAt:   "2000",
		UpdatedAt:   "2000",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/beers", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "real-user", ""))
	rr := httptest.NewRecorder()

	controller.CreateBeer(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rr.Code)
	}
	// O handler deve ter LIMPADO os campos do cliente; o preenchimento
	// com o token de sessão é responsabilidade do usecase (testado à parte).
	if captured.CreatedBy == "attacker" {
		t.Errorf("poluição: CreatedBy não foi limpo, veio do cliente (%q)", captured.CreatedBy)
	}
	if captured.ID == "forged" {
		t.Errorf("poluição: ID não foi limpo, veio do cliente (%q)", captured.ID)
	}
}

// === Invariante de Concorrência (Checklist #2, -race) ===
// Toggle de like sob rajada concorrente não deve corromper estado nem
// entrar em condição de corrida na memória do Go.

func TestLikeComment_ConcurrentToggle_NoDataRace(t *testing.T) {
	m := new(MockBeerUsecase)
	controller := NewBeerController(m, mockLogger, nil)

	// Simula um toggle atómico: conta chamadas e alterna erro/nil,
	// imitando insert/delete numa constraint única do Postgres.
	var mu sync.Mutex
	var calls int
	m.LikeCommentFunc = func(ctx context.Context, beerID, commentID, userID, deviceID string) error {
		mu.Lock()
		calls++
		odd := calls%2 == 1
		mu.Unlock()
		if odd {
			return nil // insert ok
		}
		return errors.NewAppError(400, "already liked", nil) // delete/conflito
	}

	req := httptest.NewRequest(http.MethodPost, "/beers/1/comments/1/like", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "usr_1", ""))
	req.SetPathValue("id", "1")
	req.SetPathValue("commentId", "1")

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			rr := httptest.NewRecorder()
			controller.LikeComment(rr, req)
			// aceita 200 (toggle aplicado) ou 400 (conflito de constraint) — ambos válidos
			if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
				t.Errorf("status inesperado %d", rr.Code)
			}
		}()
	}
	wg.Wait()

	if calls != n {
		t.Errorf("esperado %d chamadas ao usecase, got %d", n, calls)
	}
}

// === Contrato de status para todas as mutações (POST/PUT) ===

// Invariante de Poluição no PUT /beers/{id}: campos de criação são
// imutáveis e nunca vêm do cliente.
func TestUpdateBeer_IgnorePollutedServerFields(t *testing.T) {
	m := new(MockBeerUsecase)
	controller := NewBeerController(m, mockLogger, nil)

	var captured model.Beer
	m.On("Update", mock.Anything, "1", mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(2).(model.Beer)
		}).
		Return(nil).Once()

	abv := 5.0
	payload := model.Beer{
		ID:          "forged",
		Name:        "X IPA",
		Style:       "IPA",
		Alcohol:     &abv,
		Taste:       "Amargo",
		Aroma:       "Cítrico",
		Color:       "Âmbar",
		Body:        "Médio",
		Carbonation: "Alta",
		Finish:      "Seco",
		CreatedBy:   "attacker",
		CreatedAt:   "2000",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/beers/1", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "real-user", ""))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	controller.UpdateBeer(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if captured.ID != "1" {
		t.Errorf("poluição: ID do path = %q, want %q", captured.ID, "1")
	}
	if captured.CreatedBy == "attacker" {
		t.Errorf("poluição: CreatedBy veio do cliente (%q)", captured.CreatedBy)
	}
	if captured.CreatedAt == "2000" {
		t.Errorf("poluição: CreatedAt veio do cliente")
	}
}

func TestMutationStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       interface{}
		setupMock  func(m *MockBeerUsecase)
		wantStatus int
	}{
		{
			name:       "POST /beers -> 201",
			method:     http.MethodPost,
			path:       "/beers",
			body:       model.Beer{Name: "X IPA", Style: "IPA", Taste: "Amargo", Aroma: "Cítrico", Color: "Âmbar", Body: "Médio", Carbonation: "Alta", Finish: "Seco"},
			setupMock:  func(m *MockBeerUsecase) { m.On("Create", mock.Anything, mock.Anything).Return(nil).Once() },
			wantStatus: http.StatusCreated,
		},
		{
			name:       "PUT /beers/{id} -> 200",
			method:     http.MethodPut,
			path:       "/beers/1",
			body:       model.Beer{Name: "X IPA", Style: "IPA", Taste: "Amargo", Aroma: "Cítrico", Color: "Âmbar", Body: "Médio", Carbonation: "Alta", Finish: "Seco"},
			setupMock:  func(m *MockBeerUsecase) { m.On("Update", mock.Anything, "1", mock.Anything).Return(nil).Once() },
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST /beers/{id}/comments -> 200",
			method:     http.MethodPost,
			path:       "/beers/1/comments",
			body:       model.Comment{Text: "boa", Rating: 4},
			setupMock:  func(m *MockBeerUsecase) { m.On("AddComment", mock.Anything, "1", mock.Anything).Return(nil).Once() },
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST /beers/{id}/comments/{cid}/like -> 200",
			method:     http.MethodPost,
			path:       "/beers/1/comments/1/like",
			body:       nil,
			setupMock:  func(m *MockBeerUsecase) { m.On("LikeComment", mock.Anything, "1", "1", "usr_1", "").Return(nil).Once() },
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := new(MockBeerUsecase)
	controller := NewBeerController(m, mockLogger, nil)
			tc.setupMock(m)

			var b bytes.Buffer
			if tc.body != nil {
				_ = json.NewEncoder(&b).Encode(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, &b)
			req = req.WithContext(middleware.WithUserID(req.Context(), "usr_1", ""))
			req.SetPathValue("id", "1")
			req.SetPathValue("commentId", "1")
			rr := httptest.NewRecorder()

			switch tc.path {
			case "/beers":
				controller.CreateBeer(rr, req)
			case "/beers/1":
				controller.UpdateBeer(rr, req)
			case "/beers/1/comments":
				controller.AddComment(rr, req)
			case "/beers/1/comments/1/like":
				controller.LikeComment(rr, req)
			}

			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
		})
	}
}

// === Validação de payload malicioso / estrutural (Camada de Transporte) ===

func TestAddComment_RejectsOversizedText(t *testing.T) {
	m := new(MockBeerUsecase)
	controller := NewBeerController(m, mockLogger, nil)

	// Texto acima do limite do validator (max=2000) + tag script injetada.
	big := strings.Repeat("a", 3000)
	payload := model.Comment{Text: "<script>" + big + "</script>", Rating: 5}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/beers/1/comments", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "usr_1", ""))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	controller.AddComment(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (texto gigante deve ser rejeitado na camada de transporte)", rr.Code)
	}
	// Garante que o usecase NUNCA foi chamado com payload inválido.
	m.AssertNotCalled(t, "AddComment", mock.Anything, mock.Anything, mock.Anything)
}
