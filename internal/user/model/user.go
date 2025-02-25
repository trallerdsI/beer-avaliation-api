package model

type User struct {
	ID       string `json:"id"`
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"-" validate:"required,min=6"` // "-" means don't show in JSON
	Created  string `json:"created"`
}
