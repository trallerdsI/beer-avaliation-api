package http

import (
	"encoding/json"
	stdErrors "errors" // Renomeado para evitar conflito com o pacote de erros do projeto
	"log/slog"
	"net/http"

	"beer-review-app/internal/user/model"
	"beer-review-app/internal/user/usecase"
	appErrors "beer-review-app/pkg/errors"
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
func (c *UserController) respondError(w http.ResponseWriter, r *http.Request, err error, message string, statusCode int) {
	c.logger.ErrorContext(r.Context(), message, "err", err)
	response.SendError(w, message, statusCode, rootCause(err))
}

// rootCause devolve a mensagem da causa mais interna do erro, ou "" se ausente.
func rootCause(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (c *UserController) Register(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
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
		c.respondError(w, r, err, "Invalid input", http.StatusBadRequest)
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
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
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
		c.respondError(w, r, err, "Invalid email or password format", http.StatusBadRequest)
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
		response.SendError(w, "User ID is required", http.StatusBadRequest)
		return
	}

	user, err := c.usecase.GetProfile(r.Context(), id)
	if err != nil {
		var appErr *appErrors.AppError
		// Uso idiomático de errors.As (agora com Unwrap suportado) para
		// erros envelopados, preservando a causa original.
		if stdErrors.As(err, &appErr) && (appErr.Code == http.StatusNotFound || appErr.Is(err)) {
			response.SendError(w, "user not found", http.StatusNotFound)
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
		response.SendError(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if r.Body == nil {
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var user model.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		c.respondError(w, r, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := c.validator.Struct(&user); err != nil {
		c.respondError(w, r, err, "Invalid input", http.StatusBadRequest)
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
		response.SendError(w, "User ID is required", http.StatusBadRequest)
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
