package http

import (
	"context"
	"encoding/json"
	stdErrors "errors" // Renomeado para evitar conflito com o pacote de erros customizado
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/microcosm-cc/bluemonday"

	"beer-review-app/pkg/uuid"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/usecase"
	appErrors "beer-review-app/pkg/errors" // Alias explícito para evitar confusão
	"beer-review-app/pkg/events"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/response"
	"beer-review-app/pkg/validation"
)

// validate é um validador de structs de stack (zero-allocation por request no
// caminho crítico: o ponteiro vive no package scope, sem new() por requisição).
var (
	validate  = newValidator()
	sanitizer = bluemonday.UGCPolicy()
)

// newValidator cria o validador e regista a regra customizada https_url, que
// garante que o imageUrl (quando presente) use exclusivamente o esquema https
// — bloqueando http, javascript:, data: e outros, mitigando XSS/SSRF.
func newValidator() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("https_url", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if val == "" {
			return true // vazio é permitido (campo opcional)
		}
		u, err := url.Parse(val)
		if err != nil {
			return false
		}
		return u.Scheme == "https"
	})
	// Enums do domínio: o backend é a fonte da verdade. O app consome os
	// valores via GET /api/v1/enums e não envia entrada livre.
	_ = v.RegisterValidation("flavor", func(fl validator.FieldLevel) bool {
		return model.IsFlavor(model.Flavor(fl.Field().String()))
	})
	_ = v.RegisterValidation("aroma", func(fl validator.FieldLevel) bool {
		return model.IsAroma(model.Aroma(fl.Field().String()))
	})
	_ = v.RegisterValidation("color", func(fl validator.FieldLevel) bool {
		return model.IsColor(model.Color(fl.Field().String()))
	})
	_ = v.RegisterValidation("body", func(fl validator.FieldLevel) bool {
		return model.IsBody(model.Body(fl.Field().String()))
	})
	_ = v.RegisterValidation("carbonation", func(fl validator.FieldLevel) bool {
		return model.IsCarbonation(model.Carbonation(fl.Field().String()))
	})
	_ = v.RegisterValidation("finish", func(fl validator.FieldLevel) bool {
		return model.IsFinish(model.Finish(fl.Field().String()))
	})
	// media_type: allowlist de Content-Types aceites no upload (RFC 7578).
	_ = v.RegisterValidation("media_type", func(fl validator.FieldLevel) bool {
		switch fl.Field().String() {
		case "image/jpeg", "image/png", "image/webp":
			return true
		}
		return false
	})
	return v
}

// BeerController handles HTTP requests related to beers.
type BeerController struct {
	usecase  usecase.BeerUsecase
	logger   *slog.Logger
	eventPub events.EventStore // opcional: nil desativa publicação e leitura de eventos
}

// NewBeerController makes a new controller for beer
func NewBeerController(u usecase.BeerUsecase, logger *slog.Logger, eventPub events.EventStore) *BeerController {
	if logger == nil {
		logger = slog.Default()
	}
	return &BeerController{usecase: u, logger: logger, eventPub: eventPub}
}

// maxPageSize limita o tamanho de página para proteger o servidor contra
// pedidos com pageSize enorme (ex: 100000) que devolveriam a tabela toda.
const maxPageSize = 50

// clampPageSize garante que o pageSize esteja em [1, maxPageSize].
func clampPageSize(n int) int {
	if n < 1 {
		return 1
	}
	if n > maxPageSize {
		return maxPageSize
	}
	return n
}

func getIntParam(query url.Values, key string, defaultValue int) int {
	param := query.Get(key)
	if param == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(param)
	if err != nil {
		return defaultValue
	}
	return value
}

func handleError(w http.ResponseWriter, ctx context.Context, logger *slog.Logger, err error, message string, statusCode int) {
	logger.ErrorContext(ctx, message, "err", err)
	status := statusCode
	if appErr := (*appErrors.AppError)(nil); stdErrors.As(err, &appErr) {
		status = appErr.Code
	}
	p := appErrors.NewProblem(status, appErrors.HTTPStatusSlug(status), message)
	if trace := middleware.TraceIDFromContext(ctx); trace != "" {
		p.TraceID = trace
	}
	response.SendProblem(w, p)
}

// sendAppError converte um AppError num Problem RFC 7807 e envia ao cliente,
// preservando o código estável, detalhes granulares e ação sugerida — sem
// expor a causa interna (OWASP A05). O TraceID é injetado pelo
// RequestIDMiddleware via contexto, se disponível.
func sendAppError(w http.ResponseWriter, r *http.Request, appErr *appErrors.AppError) {
	problem := appErr.ToProblem(r.URL.Path)
	if trace := middleware.TraceIDFromContext(r.Context()); trace != "" {
		problem.TraceID = trace
	}
	response.SendProblem(w, problem)
}

// validationMessage traduz os erros do validator numa mensagem legível para o
// utilizador (ex: "nome: deve ter pelo menos 3 caracteres"), em vez de expor o
// formato cru do go-playground ("Key: 'Beer.Name' Error:...") nem as tags
// técnicas (min, https_url, flavor). O cliente (app Flutter) recebe texto
// compreensível, sem vazar detalhes de implementação (OWASP A05).
func validationMessage(err error) string {
	var verrs validator.ValidationErrors
	if stdErrors.As(err, &verrs) {
		msgs := make([]string, 0, len(verrs))
		for _, fe := range verrs {
			msgs = append(msgs, validation.FieldLabel(fe.Field())+": "+validation.RuleMessage(fe))
		}
		return strings.Join(msgs, "; ")
	}
	return err.Error()
}

// validateBeer valida o modelo completo (name, style, alcohol, url, enums)
// usando o validator já configurado no package scope, garantindo que regras de
// negócio (ex: ABV 0–100) sejam aplicadas no create/update. Em caso de falha,
// devolve um *AppError cuja Message é a mensagem de validação já traduzida
// (ex: "Name: valor inválido para a regra 'min'"), SEM o prefixo interno
// "Error 400: invalid beer:" — evitando vazar a string crua do AppError no
// corpo da resposta (OWASP A05).
func (c *BeerController) validateBeer(beer model.Beer) error {
	if err := validate.Struct(beer); err != nil {
		return appErrors.NewAppError(400, validationMessage(err), nil)
	}
	return nil
}

func (c *BeerController) GetAllBeers(w http.ResponseWriter, r *http.Request) {
	// REUTILIZADO: Uso consistente do helper 'getIntParam'
	query := r.URL.Query()
	page := getIntParam(query, "page", 1)
	pageSize := clampPageSize(getIntParam(query, "pageSize", 10))

	beers, total, err := c.usecase.GetPaginated(r.Context(), page, pageSize)
	if err != nil {
		status := http.StatusInternalServerError
		if appErr := (*appErrors.AppError)(nil); stdErrors.As(err, &appErr) {
			status = appErr.Code
		}
		handleError(w, r.Context(), c.logger, err, "Failed to retrieve beers", status)
		return
	}

	// Contrato de resposta (regressão): uma lista vazia NUNCA é serializada
	// como null. O SelectFields devolve o payload original quando não há
	// "fields", e um slice nil de Beers marshala para null. Forçamos [] aqui.
	if beers == nil {
		beers = []model.Beer{}
	}

	payload := map[string]interface{}{
		"beers":    beers,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	}

	// RFC 9111: lista paginada muda com frequência, logo ETag por conteúdo
	// (hash do payload) e Cache-Control curto. 304 evita reenvio do body.
	body, err := json.Marshal(payload)
	if err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to encode beers", http.StatusInternalServerError)
		return
	}
	etag := response.ETagForContent(body)
	if response.IfNoneMatchMatches(r, etag) {
		response.SendNotModified(w, etag)
		return
	}
	response.SetCacheHeaders(w, etag, 30, true)
	setPaginationLinks(w, r, page, pageSize, total, "/api/v1/beers")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (c *BeerController) CreateBeer(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		handleError(w, r.Context(), c.logger, fmt.Errorf("request body is empty"), "Invalid request body", http.StatusBadRequest)
		return
	}

	// SEGURANÇA: Limita o tamanho do JSON para 1MB evitando Denial of Service (DoS) por memória excedida
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var beer model.Beer
	if err := json.NewDecoder(r.Body).Decode(&beer); err != nil {
		handleError(w, r.Context(), c.logger, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	beer.Comments = make([]model.Comment, 0)
	// Invariante de Poluição: campos protegidos NUNCA vêm do cliente. O
	// servidor é a única fonte de id/autor/timestamps (o usecase regera a
	// partir do token de sessão). Limpar previne forja de createdBy/id.
	beer.ID = ""
	beer.CreatedBy = ""
	beer.CreatedAt = ""
	beer.UpdatedAt = ""
	beer.Name = sanitizer.Sanitize(beer.Name)
	beer.Description = sanitizer.Sanitize(beer.Description)

	if err := c.validateBeer(beer); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Invalid beer", http.StatusBadRequest)
		return
	}

	if err := c.usecase.Create(r.Context(), &beer); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to create beer", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusCreated, beer)

	c.logger.Info("Beer created", slog.String("name", beer.Name), slog.String("style", beer.Style))
}

func (c *BeerController) UpdateBeer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if r.Body == nil {
		handleError(w, r.Context(), c.logger, fmt.Errorf("request body is empty"), "Invalid request body", http.StatusBadRequest)
		return
	}

	// SEGURANÇA: Limitação de carga útil
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var beer model.Beer
	if err := json.NewDecoder(r.Body).Decode(&beer); err != nil {
		handleError(w, r.Context(), c.logger, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	beer.ID = id // vem do path, nunca do body
	// Invariante de Poluição: o autor/timestamps de criação são imutáveis e
	// nunca vêm do cliente num PUT — mantêm-se os originais da BD.
	beer.CreatedBy = ""
	beer.CreatedAt = ""
	beer.Name = sanitizer.Sanitize(beer.Name)
	beer.Description = sanitizer.Sanitize(beer.Description)

	if err := c.validateBeer(beer); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Invalid beer", http.StatusBadRequest)
		return
	}

	if err := c.usecase.Update(r.Context(), id, beer); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to update beer", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, beer)
}

func (c *BeerController) DeleteBeer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := c.usecase.Delete(r.Context(), id); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to delete beer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *BeerController) GetBeerByID(w http.ResponseWriter, r *http.Request) {
	beerID := r.PathValue("id")

	beer, err := c.usecase.GetByID(r.Context(), beerID)
	if err != nil {
		var appErr *appErrors.AppError
		// CORRIGIDO: Uso moderno de errors.As para segurança e compatibilidade com wrapping
		if stdErrors.As(err, &appErr) && (appErr.Code == http.StatusNotFound || strings.Contains(strings.ToLower(appErr.Detail), "not found")) {
			// PADRONIZADO: envelope JSON consistente com os restantes erros
			// do controller (response.SendError), em vez de texto plano.
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to retrieve beer", http.StatusInternalServerError)
		return
	}

	// RFC 9111: ETag por updated_at (versão estável). Se o cliente reenvia
	// If-None-Match e bate, responde 304 sem body (poupa banda no mobile).
	etag := response.ETagForVersion(beer.ID + ":" + beer.UpdatedAt)
	if response.IfNoneMatchMatches(r, etag) {
		response.SendNotModified(w, etag)
		return
	}
	response.SetCacheHeaders(w, etag, 300, true)

	// PADRONIZADO: Uso do helper global de respostas JSON em vez de encode cru manual
	response.SendResponse(w, http.StatusOK, beer)
}

func (c *BeerController) AddComment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if r.Body == nil {
		handleError(w, r.Context(), c.logger, fmt.Errorf("request body is empty"), "Invalid request body", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var comment model.Comment
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		handleError(w, r.Context(), c.logger, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	// SEGURANÇA: sanitiza o texto contra XSS ANTES de validar, para que a
	// regra "required" confira o conteúdo já limpo (um texto puramente
	// <script> vira "" e é rejeitado com 400, não gravado vazio).
	comment.Text = sanitizer.Sanitize(comment.Text)

	if err := validate.Struct(comment); err != nil {
		handleError(w, r.Context(), c.logger, err, "Invalid input", http.StatusBadRequest)
		return
	}

	comment.ID = uuid.MustNewV7()
	comment.Likes = 0
	// AuthZ: regista o autor do comentário (qualquer user logado pode comentar).
	if uid, ok := middleware.UserIDFromContext(r.Context()); ok {
		comment.CreatedBy = uid
	}
	// Timestamp para ordenação cronológica do feed social.
	comment.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := c.usecase.AddComment(r.Context(), id, comment); err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to add comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message":   "Comment added successfully",
		"commentId": comment.ID,
	})
}

func (c *BeerController) DeleteComment(w http.ResponseWriter, r *http.Request) {
	beerID := r.PathValue("id")
	commentID := r.PathValue("commentId")

	if err := c.usecase.DeleteComment(r.Context(), beerID, commentID); err != nil {
		var appErr *appErrors.AppError
		// respeita o status do AppError (ex: 403 em AuthZ, 404 not found).
		if stdErrors.As(err, &appErr) {
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to delete comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{"message": "Comment deleted successfully"})
}

func (c *BeerController) LikeComment(w http.ResponseWriter, r *http.Request) {
	beerID := r.PathValue("id")
	commentID := r.PathValue("commentId")

	// Like exige login (middleware.Auth garante userID). Sem fallback anónimo
	// (Decisão A): evita spam/bots em interações sociais.
	userID, _ := middleware.UserIDFromContext(r.Context())

	if err := c.usecase.LikeComment(r.Context(), beerID, commentID, userID, ""); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to like comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{"message": "Comment liked successfully"})
}

func (c *BeerController) SearchBeers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if len(query.Get("q")) > maxSearchQueryLength {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "invalid_request",
			fmt.Sprintf("Search query exceeds maximum length of %d characters", maxSearchQueryLength)))
		return
	}

	filters := model.BeerFilters{
		Query:    query.Get("q"),
		Style:    query.Get("style"),
		Taste:    query.Get("taste"),
		Page:     getIntParam(query, "page", 1),
		PageSize: clampPageSize(getIntParam(query, "limit", 10)),
	}

	if fuzzy := query.Get("fuzzy"); fuzzy != "" {
		if val, err := strconv.ParseBool(fuzzy); err == nil {
			filters.Fuzzy = &val
		}
	}

	if minAlc := query.Get("minAlcohol"); minAlc != "" {
		if f, err := strconv.ParseFloat(minAlc, 64); err == nil {
			filters.MinAlcohol = &f
		}
	}

	if maxAlc := query.Get("maxAlcohol"); maxAlc != "" {
		if f, err := strconv.ParseFloat(maxAlc, 64); err == nil {
			filters.MaxAlcohol = &f
		}
	}

	beers, total, fuzzyMatch, err := c.usecase.SearchBeers(r.Context(), filters)
	if err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to search beers", http.StatusInternalServerError)
		return
	}

	// Contrato: lista vazia nunca como null.
	if beers == nil {
		beers = []model.Beer{}
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"beers":    beers,
		"total":    total,
		"page":     filters.Page,
		"pageSize": filters.PageSize,
		"meta": map[string]interface{}{
			"fuzzy_match": fuzzyMatch,
		},
	})
}

// GetEnums devolve os valores aceites para os enums do domínio (style, taste,
// aroma, color, body, carbonation, finish). O backend é a fonte da verdade:
// o app Flutter consome esta lista e não aceita entrada livre do utilizador.
// RFC 9111: lista quase estática do domínio — ETag por conteúdo e cache longo
// (1h), poupando polling do app móvel.
func (c *BeerController) GetEnums(w http.ResponseWriter, r *http.Request) {
	payload := model.EnumValues()
	body, err := json.Marshal(payload)
	if err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to encode enums", http.StatusInternalServerError)
		return
	}
	etag := response.ETagForContent(body)
	if response.IfNoneMatchMatches(r, etag) {
		response.SendNotModified(w, etag)
		return
	}
	response.SetCacheHeaders(w, etag, 3600, true)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payload)
}

// maxSearchQueryLength limita o tamanho da query FTS para evitar DoS por
// consumo excessivo de CPU no PostgreSQL (pg_trgm / tsvector).
const maxSearchQueryLength = 200

// UploadBeerMedia recebe uma imagem via multipart/form-data (RFC 7578), valida
// o binário por magic bytes (não só extensão), faz upload para o object storage
// (Supabase Storage removido; upload não está disponível nesta versão).
func (c *BeerController) UploadBeerMedia(w http.ResponseWriter, r *http.Request) {
	response.SendProblem(w, appErrors.NewProblem(http.StatusNotImplemented, "media_disabled",
		"Upload de mídia não está disponível."))
}

func (c *BeerController) ListBeerEvents(w http.ResponseWriter, r *http.Request) {
	beerID := r.PathValue("id")
	if beerID == "" {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "invalid_request", "beer id is required"))
		return
	}

	since := r.URL.Query().Get("since")
	var sinceTime time.Time
	if since != "" {
		if ts, err := strconv.ParseInt(since, 10, 64); err == nil {
			sinceTime = time.Unix(ts, 0)
		}
	}

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Vary", "Accept-Encoding, If-None-Match")

	if c.eventPub != nil {
		_, latestMember, err := c.eventPub.LatestEvent(r.Context(), beerID)
		if err != nil {
			handleError(w, r.Context(), c.logger, err, "Failed to get latest event", http.StatusInternalServerError)
			return
		}

		etag := events.SanitizeETag(latestMember)
		w.Header().Set("ETag", etag)
		if match := r.Header.Get("If-None-Match"); match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	evts, err := c.usecase.ListBeerEvents(r.Context(), beerID, sinceTime)
	if err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to list events", http.StatusInternalServerError)
		return
	}
	if evts == nil {
		evts = []model.BeerEvent{}
	}
	if len(evts) > 0 {
		latest := evts[len(evts)-1]
		etag := events.SanitizeETag(latest.ID)
		w.Header().Set("ETag", etag)
		if match := r.Header.Get("If-None-Match"); match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	response.SendResponse(w, http.StatusOK, map[string]any{
		"events": evts,
		"since":  sinceTime.Unix(),
	})
}
