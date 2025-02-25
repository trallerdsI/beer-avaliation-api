package http

import (
	"encoding/json"
	"net/http"

	"beer-review-app/internal/user/model"
	"beer-review-app/internal/user/usecase"
	"beer-review-app/pkg/response"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

type UserController struct {
	usecase usecase.UserUsecase
	logger  *zap.Logger
}

func NewUserController(u usecase.UserUsecase, logger *zap.Logger) *UserController {
	return &UserController{usecase: u, logger: logger}
}

// @Summary Register new user
// @Description Register a new user account
// @Accept json
// @Produce json
// @Param user body model.User true "User Registration Info"
// @Success 201 {object} model.User
// @Failure 400 {object} map[string]string
// @Router /users/register [post]
func (c *UserController) Register(w http.ResponseWriter, r *http.Request) {
	var user model.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		c.logger.Warn("Invalid request body", zap.Error(err))
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := validate.Struct(user); err != nil {
		c.logger.Warn("Invalid input", zap.Error(err))
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

func (c *UserController) Login(w http.ResponseWriter, r *http.Request) {
	var credentials struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		c.logger.Warn("Invalid request body", zap.Error(err))
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
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

func (c *UserController) GetProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	user, err := c.usecase.GetProfile(r.Context(), id)
	if err != nil {
		c.logger.Error("Failed to get profile", zap.Error(err))
		response.SendError(w, "Failed to get profile", http.StatusNotFound)
		return
	}

	response.SendResponse(w, http.StatusOK, user)
}

func (c *UserController) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var user model.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		c.logger.Warn("Invalid request body", zap.Error(err))
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
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

func (c *UserController) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := c.usecase.DeleteAccount(r.Context(), id); err != nil {
		c.logger.Error("Failed to delete account", zap.Error(err))
		response.SendError(w, "Failed to delete account", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message": "Account deleted successfully",
	})
}

// Add other handler methods...
