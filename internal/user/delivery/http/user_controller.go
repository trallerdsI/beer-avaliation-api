package http

import (
	"encoding/json"
	stdErrors "errors" // Renomeado para evitar conflito com o pacote de erros do projeto
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"beer-review-app/internal/user/model"
	"beer-review-app/internal/user/usecase"
	appErrors "beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/response"

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
	// RFC 7807: erro genérico (sem causa interna) com trace_id para suporte.
	p := response.NewProblem(statusCode, appErrors.HTTPStatusSlug(statusCode), message)
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
			Field:   fieldLabel(fe.Field()),
			Code:    "invalid_" + fe.Tag(),
			Message: fmt.Sprintf("%s: %s", fieldLabel(fe.Field()), ruleMessage(fe)),
		})
	}
	return details
}
// sendValidationProblem envia um Problem RFC 7807 de validação (code
// "validation_failed") com detalhes granulares tipados e trace_id.
func (c *UserController) sendValidationProblem(w http.ResponseWriter, r *http.Request, err error) {
	c.logger.ErrorContext(r.Context(), "validation failed", "err", err)
	p := response.NewProblem(http.StatusBadRequest, "validation_failed",
		"Os dados enviados contêm erros de validação.")
	p.Details = validationDetails(err)
	if trace := middleware.TraceIDFromContext(r.Context()); trace != "" {
		p.TraceID = trace
	}
	response.SendProblem(w, p)
}


func fieldLabel(field string) string {
	labels := map[string]string{
		"Username": "nome de utilizador",
		"Email":    "email",
		"Password": "palavra-passe",
		"Role":     "perfil",
	}
	if l, ok := labels[field]; ok {
		return l
	}
	return strings.ToLower(field)
}

func ruleMessage(fe validator.FieldError) string {
	param := fe.Param()
	switch fe.Tag() {
	case "required":
		return "é obrigatório"
	case "min":
		if isNumericKind(fe.Kind()) {
			return fmt.Sprintf("deve ter no mínimo %s", param)
		}
		return fmt.Sprintf("deve ter pelo menos %s caracteres", param)
	case "max":
		if isNumericKind(fe.Kind()) {
			return fmt.Sprintf("deve ter no máximo %s", param)
		}
		return fmt.Sprintf("deve ter no máximo %s caracteres", param)
	case "email":
		return "deve ser um email válido"
	default:
		return "valor inválido"
	}
}

func isNumericKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func (c *UserController) Register(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		response.SendProblem(w, response.NewProblem(http.StatusBadRequest, "invalid_request_body", "O corpo da requisição é inválido."))
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
			c.respondError(w, r, err, appErr.Message, appErr.Code)
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
		response.SendProblem(w, response.NewProblem(http.StatusBadRequest, "invalid_request_body", "O corpo da requisição é inválido."))
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

func (c *UserController) GetProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		response.SendProblem(w, response.NewProblem(http.StatusBadRequest, "bad_request", "O ID do utilizador é obrigatório."))
		return
	}

	user, err := c.usecase.GetProfile(r.Context(), id)
	if err != nil {
		var appErr *appErrors.AppError
		// Uso idiomático de errors.As (agora com Unwrap suportado) para
		// erros envelopados, preservando a causa original.
		if stdErrors.As(err, &appErr) && (appErr.Code == http.StatusNotFound || appErr.Is(err)) {
			response.SendProblem(w, response.NewProblem(http.StatusNotFound, "not_found", "Utilizador não encontrado."))
			return
		}
		c.respondError(w, r, err, "Failed to get profile", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, user)
}

func (c *UserController) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		response.SendProblem(w, response.NewProblem(http.StatusBadRequest, "bad_request", "O ID do utilizador é obrigatório."))
		return
	}

	if r.Body == nil {
		response.SendProblem(w, response.NewProblem(http.StatusBadRequest, "invalid_request_body", "O corpo da requisição é inválido."))
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
		response.SendProblem(w, response.NewProblem(http.StatusBadRequest, "bad_request", "O ID do utilizador é obrigatório."))
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
