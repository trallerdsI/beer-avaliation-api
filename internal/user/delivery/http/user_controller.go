package http

import (
	"encoding/json"
	stdErrors "errors" // Renomeado para evitar conflito com o pacote de erros do projeto
	"log/slog"
	"net/http"

	"beer-review-app/internal/user/model"
	"beer-review-app/internal/user/usecase"
	appErrors "beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/response"
	"beer-review-app/pkg/validation"

	"github.com/go-playground/validator/v10"
)

// UserController handles HTTP requests related to users.
type UserController struct {
	usecase   usecase.UserUsecase
	logger    *slog.Logger
	validator *validator.Validate // Validator encapsulado na struct para evitar colisão de pacotes
}

// NewUserController makes a new controller for user
func NewUserController(u usecase.UserUsecase, logger *slog.Logger) *UserController {
	if logger == nil {
		logger = slog.Default()
	}
	return &UserController{
		usecase:   u,
		logger:    logger,
		validator: validator.New(), // Inicializado de forma segura e isolada
	}
}

// respondError escreve um erro JSON e registra o evento com contexto da
// requisição (traceability). Centraliza o tratamento para reduzir boilerplate.
// Segurança (OWASP A05): não expõe a causa raiz ao cliente.
func (c *UserController) respondError(w http.ResponseWriter, r *http.Request, err error, message string, statusCode int) {
	c.logger.ErrorContext(r.Context(), message, "err", err)
	status := statusCode
	if appErr := (*appErrors.AppError)(nil); stdErrors.As(err, &appErr) {
		status = appErr.Code
	}
	p := appErrors.NewProblem(status, appErrors.HTTPStatusSlug(status), message)
	if trace := middleware.TraceIDFromContext(r.Context()); trace != "" {
		p.TraceID = trace
	}
	response.SendProblem(w, p)
}

// validationDetails converte os erros do validator em detalhes granulares
// tipados (RFC 7807), legíveis e sem expor tags técnicas (min, email) nem o
// formato cru do go-playground.
func validationDetails(err error) []appErrors.ProblemDetail {
	var verrs validator.ValidationErrors
	if !stdErrors.As(err, &verrs) {
		return nil
	}
	details := make([]appErrors.ProblemDetail, 0, len(verrs))
	for _, fe := range verrs {
		details = append(details, appErrors.ProblemDetail{
			Field:   validation.FieldLabel(fe.Field()),
			Code:    "invalid_" + fe.Tag(),
			Detail:  validation.FieldLabel(fe.Field()) + ": " + validation.RuleMessage(fe),
		})
	}
	return details
}

// sendValidationProblem envia um Problem RFC 7807 de validação (code
// "validation_failed") com detalhes granulares tipados e trace_id.
func (c *UserController) sendValidationProblem(w http.ResponseWriter, r *http.Request, err error) {
	c.logger.ErrorContext(r.Context(), "validation failed", "err", err)
	p := appErrors.NewProblem(http.StatusBadRequest, "validation_failed",
		"Os dados enviados contêm erros de validação.")
	p.Details = validationDetails(err)
	if trace := middleware.TraceIDFromContext(r.Context()); trace != "" {
		p.TraceID = trace
	}
	response.SendProblem(w, p)
}



func (c *UserController) Register(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "invalid_request_body", "O corpo da requisição é inválido."))
		return
	}

	// SEGURANÇA: Limita o payload de registro para evitar ataques de DoS de memória
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // Limitado a 1MB

	var user model.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		c.respondError(w, r, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := c.validator.Struct(&user); err != nil {
		c.sendValidationProblem(w, r, err)
		return
	}

	if err := c.usecase.Register(r.Context(), user); err != nil {
		if appErr := (*appErrors.AppError)(nil); stdErrors.As(err, &appErr) {
			c.respondError(w, r, err, appErr.Detail, appErr.Code)
			return
		}
		c.respondError(w, r, err, "Failed to register user", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusCreated, map[string]string{
		"message": "User registered successfully",
	})
}

func (c *UserController) Login(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "invalid_request_body", "O corpo da requisição é inválido."))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var credentials struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		c.respondError(w, r, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validação de login executada antes de ir para o usecase.
	if err := c.validator.Struct(&credentials); err != nil {
		c.sendValidationProblem(w, r, err)
		return
	}

	token, err := c.usecase.Login(r.Context(), credentials.Email, credentials.Password)
	if err != nil {
		// Não vazamos a causa interna (credenciais) no corpo da resposta.
		c.respondError(w, r, err, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

func (c *UserController) OAuth(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "invalid_request_body", "O corpo da requisição é inválido."))
		return
	}

	// Limita o payload (o id_token é pequeno, mas mantemos o teto defensivo).
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var body struct {
		Provider string `json:"provider" validate:"required"`
		IDToken  string `json:"id_token" validate:"required"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		c.respondError(w, r, err, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := c.validator.Struct(body); err != nil {
		c.sendValidationProblem(w, r, err)
		return
	}

	token, err := c.usecase.OAuthLogin(r.Context(), body.Provider, body.IDToken)
	if err != nil {
		// Não vazamos a causa interna (falha de validação do IdP).
		c.respondError(w, r, err, "OAuth login failed", http.StatusUnauthorized)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

func (c *UserController) GetProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "bad_request", "O ID do utilizador é obrigatório."))
		return
	}

	user, err := c.usecase.GetProfile(r.Context(), id)
	if err != nil {
		var appErr *appErrors.AppError
		// Uso idiomático de errors.As (agora com Unwrap suportado) para
		// erros envelopados, preservando a causa original.
		if stdErrors.As(err, &appErr) && (appErr.Code == http.StatusNotFound || appErr.Is(err)) {
			response.SendProblem(w, appErrors.NewProblem(http.StatusNotFound, "not_found", "Utilizador não encontrado."))
			return
		}
		c.respondError(w, r, err, "Failed to get profile", http.StatusInternalServerError)
		return
	}

	// RFC 9111: ETag por updated_at (versão estável do perfil).
	etag := response.ETagForVersion(user.ID + ":" + user.UpdatedAt)
	if response.IfNoneMatchMatches(r, etag) {
		response.SendNotModified(w, etag)
		return
	}
	response.SetCacheHeaders(w, etag, 120, true)

	response.SendResponse(w, http.StatusOK, user)
}

func (c *UserController) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "bad_request", "O ID do utilizador é obrigatório."))
		return
	}

	if r.Body == nil {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "invalid_request_body", "O corpo da requisição é inválido."))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var user model.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		c.respondError(w, r, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := c.validator.Struct(&user); err != nil {
		c.sendValidationProblem(w, r, err)
		return
	}

	// Invariante de Poluição / Escalonamento de Privilégio: o id vem do path e
	// o role é imutável pelo próprio utilizador (apenas admin via seed). O
	// cliente NUNCA pode promover-se a admin ou alterar o dono do recurso.
	user.ID = ""
	user.Role = ""

	if err := c.usecase.UpdateProfile(r.Context(), id, user); err != nil {
		c.respondError(w, r, err, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message": "Profile updated successfully",
	})
}

func (c *UserController) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "bad_request", "O ID do utilizador é obrigatório."))
		return
	}

	if err := c.usecase.DeleteAccount(r.Context(), id); err != nil {
		c.respondError(w, r, err, "Failed to delete account", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message": "Account deleted successfully",
	})
}

func (c *UserController) SubscribePush(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "bad_request", "O ID do utilizador é obrigatório."))
		return
	}

	if r.Body == nil {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "invalid_request_body", "O corpo da requisição é inválido."))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var sub model.PushSubscription
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		c.respondError(w, r, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	if sub.Endpoint == "" || sub.P256DH == "" || sub.Auth == "" {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "invalid_push_subscription",
			"endpoint, p256dh e auth são obrigatórios."))
		return
	}

	if err := c.usecase.SubscribePush(r.Context(), id, sub); err != nil {
		c.respondError(w, r, err, "Failed to subscribe push", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusCreated, map[string]string{
		"message": "Push subscription created",
	})
}

func (c *UserController) UnsubscribePush(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "bad_request", "O ID do utilizador é obrigatório."))
		return
	}

	if r.Body == nil {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "invalid_request_body", "O corpo da requisição é inválido."))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		c.respondError(w, r, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.Endpoint == "" {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "invalid_request_body",
			"endpoint é obrigatório."))
		return
	}

	if err := c.usecase.UnsubscribePush(r.Context(), id, body.Endpoint); err != nil {
		c.respondError(w, r, err, "Failed to unsubscribe push", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message": "Push subscription removed",
	})
}

func (c *UserController) ListPushSubscriptions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		response.SendProblem(w, appErrors.NewProblem(http.StatusBadRequest, "bad_request", "O ID do utilizador é obrigatório."))
		return
	}

	subs, err := c.usecase.ListPushSubscriptions(r.Context(), id)
	if err != nil {
		c.respondError(w, r, err, "Failed to list push subscriptions", http.StatusInternalServerError)
		return
	}

	if subs == nil {
		subs = []model.PushSubscription{}
	}
	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"subscriptions": subs,
	})
}
