package http

import (
	"context"
	"encoding/json"
	stdErrors "errors" // Renomeado para evitar conflito com o pacote de erros customizado
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/usecase"
	appErrors "beer-review-app/pkg/errors" // Alias explícito para evitar confusão
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/response"
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
	v.RegisterValidation("https_url", func(fl validator.FieldLevel) bool {
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
	v.RegisterValidation("flavor", func(fl validator.FieldLevel) bool {
		return model.IsFlavor(model.Flavor(fl.Field().String()))
	})
	v.RegisterValidation("aroma", func(fl validator.FieldLevel) bool {
		return model.IsAroma(model.Aroma(fl.Field().String()))
	})
	v.RegisterValidation("color", func(fl validator.FieldLevel) bool {
		return model.IsColor(model.Color(fl.Field().String()))
	})
	v.RegisterValidation("body", func(fl validator.FieldLevel) bool {
		return model.IsBody(model.Body(fl.Field().String()))
	})
	v.RegisterValidation("carbonation", func(fl validator.FieldLevel) bool {
		return model.IsCarbonation(model.Carbonation(fl.Field().String()))
	})
	v.RegisterValidation("finish", func(fl validator.FieldLevel) bool {
		return model.IsFinish(model.Finish(fl.Field().String()))
	})
	return v
}

// BeerController handles HTTP requests related to beers.
type BeerController struct {
	usecase usecase.BeerUsecase
	logger  *slog.Logger
}

// NewBeerController makes a new controller for beer
func NewBeerController(u usecase.BeerUsecase, logger *slog.Logger) *BeerController {
	if logger == nil {
		logger = slog.Default()
	}
	return &BeerController{usecase: u, logger: logger}
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
	// Segurança (OWASP A05): nunca expõe a causa raiz (ex: erro de DB) ao
	// cliente — apenas regista no log do servidor. O cliente recebe a
	// mensagem genérica.
	response.SendError(w, message, statusCode)
}

// sendAppError responde com o status/mensagem do AppError e, se presentes,
// propaga o código de erro estável e o detail seguro (ex: sugestões de
// duplicado) para o cliente — sem expor a causa interna.
func sendAppError(w http.ResponseWriter, appErr *appErrors.AppError) {
	opts := []response.ErrorOption{}
	if appErr.ErrorCode != "" {
		opts = append(opts, response.WithErrorCode(appErr.ErrorCode))
	}
	if appErr.Detail != nil {
		opts = append(opts, response.WithDetail(appErr.Detail))
	}
	response.SendError(w, appErr.Message, appErr.Code, opts...)
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
			msgs = append(msgs, fmt.Sprintf("%s: %s", fieldLabel(fe.Field()), ruleMessage(fe)))
		}
		return strings.Join(msgs, "; ")
	}
	return err.Error()
}

// fieldLabel traduz o nome do campo Go para um rótulo legível em PT.
func fieldLabel(field string) string {
	labels := map[string]string{
		"Name":        "nome",
		"Style":       "estilo",
		"Description": "descrição",
		"ImageUrl":    "imagem",
		"Alcohol":     "teor alcoólico",
		"Taste":       "sabor",
		"Aroma":       "aroma",
		"Color":       "cor",
		"Body":        "corpo",
		"Carbonation": "carbonatação",
		"Finish":      "finalização",
		"Text":        "texto",
		"Rating":      "avaliação",
		"Comments":    "comentários",
	}
	if l, ok := labels[field]; ok {
		return l
	}
	return strings.ToLower(field)
}

// ruleMessage gera uma frase clara para a regra violada, incluindo o valor
// esperado (fe.Param) quando aplicável.
func ruleMessage(fe validator.FieldError) string {
	param := fe.Param()
	switch fe.Tag() {
	case "required":
		return "é obrigatório"
	case "min":
		if isNumericKind(fe.Kind()) {
			return fmt.Sprintf("deve ser no mínimo %s", param)
		}
		return fmt.Sprintf("deve ter pelo menos %s caracteres", param)
	case "max":
		if isNumericKind(fe.Kind()) {
			return fmt.Sprintf("deve ser no máximo %s", param)
		}
		return fmt.Sprintf("deve ter no máximo %s caracteres", param)
	case "https_url":
		return "deve ser um URL https válido"
	case "email":
		return "deve ser um email válido"
	default:
		// Regras custom de enum (flavor, aroma, color, body, carbonation, finish):
		// não expõem a tag técnica, apenas indicam valor inválido.
		return "valor inválido"
	}
}

// isNumericKind indica se o kind do campo é inteiro ou float (para distinguir
// "caracteres" de "valor numérico" nas regras min/max).
func isNumericKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
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
		handleError(w, r.Context(), c.logger, err, "Failed to retrieve beers", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"beers":    response.SelectFields(beers, query.Get("fields")),
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
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
	beer.Name = sanitizer.Sanitize(beer.Name)
	beer.Description = sanitizer.Sanitize(beer.Description)

	if err := c.validateBeer(beer); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Invalid beer", http.StatusBadRequest)
		return
	}

	if err := c.usecase.Create(r.Context(), &beer); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, appErr)
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

	beer.ID = id
	beer.Name = sanitizer.Sanitize(beer.Name)
	beer.Description = sanitizer.Sanitize(beer.Description)

	if err := c.validateBeer(beer); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Invalid beer", http.StatusBadRequest)
		return
	}

	if err := c.usecase.Update(r.Context(), id, beer); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, appErr)
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
			sendAppError(w, appErr)
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
		if stdErrors.As(err, &appErr) && (appErr.Code == http.StatusNotFound || strings.Contains(strings.ToLower(appErr.Message), "not found")) {
			// PADRONIZADO: envelope JSON consistente com os restantes erros
			// do controller (response.SendError), em vez de texto plano.
			sendAppError(w, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to retrieve beer", http.StatusInternalServerError)
		return
	}

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

	comment.ID = uuid.New().String()
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
			sendAppError(w, appErr)
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
			sendAppError(w, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to like comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{"message": "Comment liked successfully"})
}

func (c *BeerController) SearchBeers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filters := model.BeerFilters{
		Query:    query.Get("query"),
		Style:    query.Get("style"),
		Taste:    query.Get("taste"),
		Page:     getIntParam(query, "page", 1),
		PageSize: clampPageSize(getIntParam(query, "pageSize", 10)),
	}

	if minAlc := query.Get("minAlcohol"); minAlc != "" {
		if val, err := strconv.ParseFloat(minAlc, 64); err == nil {
			filters.MinAlcohol = new(float64(val))
		}
	}

	if maxAlc := query.Get("maxAlcohol"); maxAlc != "" {
		if val, err := strconv.ParseFloat(maxAlc, 64); err == nil {
			filters.MaxAlcohol = new(float64(val))
		}
	}

	beers, total, err := c.usecase.SearchBeers(r.Context(), filters)
	if err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to search beers", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"beers":    beers,
		"total":    total,
		"page":     filters.Page,
		"pageSize": filters.PageSize,
	})
}

// GetEnums devolve os valores aceites para os enums do domínio (style, taste,
// aroma, color, body, carbonation, finish). O backend é a fonte da verdade:
// o app Flutter consome esta lista e não aceita entrada livre do utilizador.
func (c *BeerController) GetEnums(w http.ResponseWriter, r *http.Request) {
	response.SendResponse(w, http.StatusOK, model.EnumValues())
}
