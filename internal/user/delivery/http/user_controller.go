package http

import (
	stdErrors "errors" // Renomeado para evitar conflito com o pacote de erros do projeto
	"encoding/json"
	"net/http"
	"strings"

	"beer-review-app/internal/user/model"
	"beer-review-app/internal/user/usecase"
	appErrors "beer-review-app/pkg/errors"
	"beer-review-app/pkg/response"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// UserController handles HTTP requests related to users.
type UserController struct {
	usecase   usecase.UserUsecase
	logger    *zap.Logger
	validator *validator.Validate // CORRIGIDO: Validator encapsulado na struct para evitar colisão de pacotes
}

// NewUserController makes a new controller for user
func NewUserController(u usecase.UserUsecase, logger *zap.Logger) *UserController {
	return &UserController{
		usecase:   u,
		logger:    logger,
		validator: validator.New(), // Inicializado de forma segura e isolada
	}
}

// @Summary Register new user
// @Description Register a new user account
// @Accept json
// @Produce json
// @Param user body model.User true "User Registration Info"
// @Success 201 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/register [post]
func (c *UserController) Register(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		c.logger.Warn("Request body is empty")
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// SEGURANÇA: Limita o payload de registro para evitar ataques de DoS de memória
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // Limitado a 1MB

	var user model.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		c.logger.Warn("Invalid request body", zap.Error(err))
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := c.validator.Struct(&user); err != nil {
		c.logger.Warn("Invalid input for registration", zap.Error(err))
		response.SendError(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if err := c.usecase.Register(r.Context(), user); err != nil {
		c.logger.Error("Failed to register user", zap.Error(err))
		response.SendError(w, "Failed to register user", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusCreated, map[string]string{
		"message": "User registered successfully",
	})
}

// @Summary User login
// @Description Authenticate user and return JWT token
// @Accept json
// @Produce json
// @Param credentials body struct{Email string `json:"email" validate:"required,email"`; Password string `json:"password" validate:"required"`} true "User Login Credentials"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /users/login [post]
func (c *UserController) Login(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		c.logger.Warn("Request body is empty")
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var credentials struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		c.logger.Warn("Invalid request body", zap.Error(err))
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// CORRIGIDO: A validação de login agora é de fato executada antes de ir para o usecase
	if err := c.validator.Struct(&credentials); err != nil {
		c.logger.Warn("Invalid login credentials format", zap.Error(err))
		response.SendError(w, "Invalid email or password format", http.StatusBadRequest)
		return
	}

	token, err := c.usecase.Login(r.Context(), credentials.Email, credentials.Password)
	if err != nil {
		c.logger.Error("Failed to login", zap.Error(err))
		response.SendError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

// @Summary Get user profile
// @Description Retrieve a user's profile by ID
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} model.User
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/{id} [get]
func (c *UserController) GetProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		response.SendError(w, "User ID is required", http.StatusBadRequest)
		return
	}

	user, err := c.usecase.GetProfile(r.Context(), id)
	if err != nil {
		var appErr *appErrors.AppError
		// CORRIGIDO: Uso idiomático de errors.As para suportar erros envelopados
		if stdErrors.As(err, &appErr) && (appErr.Code == http.StatusNotFound || strings.Contains(strings.ToLower(appErr.Message), "not found")) {
			response.SendError(w, appErr.Message, http.StatusNotFound)
			return
		}
		c.logger.Error("Failed to get profile", zap.Error(err))
		response.SendError(w, "Failed to get profile", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, user)
}

// @Summary Update user profile
// @Description Update user details by ID
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param user body model.User true "Updated User Info"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/{id} [put]
func (c *UserController) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
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
		c.logger.Warn("Invalid request body", zap.Error(err))
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// CORRIGIDO: Valida os novos dados do perfil antes de submetê-los para persistência
	if err := c.validator.Struct(&user); err != nil {
		c.logger.Warn("Invalid profile update input", zap.Error(err))
		response.SendError(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if err := c.usecase.UpdateProfile(r.Context(), id, user); err != nil {
		c.logger.Error("Failed to update profile", zap.Error(err))
		response.SendError(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message": "Profile updated successfully",
	})
}

// @Summary Delete user account
// @Description Delete user account by ID
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/{id} [delete]
func (c *UserController) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		response.SendError(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if err := c.usecase.DeleteAccount(r.Context(), id); err != nil {
		c.logger.Error("Failed to delete account", zap.Error(err))
		response.SendError(w, "Failed to delete account", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message": "Account deleted successfully",
	})
}