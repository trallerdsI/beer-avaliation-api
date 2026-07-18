package model

// Roles de utilizador. Admin tem escopo global (override de dono em beers/comentários).
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type User struct {
	ID       string `json:"id"`
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"` // "-" means don't show in JSON
	Role     string `json:"role"`
	Created  string `json:"-"`
	UpdatedAt string `json:"-"` // base do ETag (RFC 9111); não exposto no JSON do perfil
}
