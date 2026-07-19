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
	// Password é recebido no JSON de registo/login (json:"password") mas
	// NUNCA devolvido nas respostas: o GetByID/GetProfile não o selecionam no
	// repositório, logo a coluna sensível (hash) não chega ao cliente. O
	// Supabase Advisor de "Sensitive Columns Exposed" é mitigado porque a API
	// não devolve o hash; o RLS em enable_rls.sql protege acesso direto à BD.
	Password  string `json:"password" validate:"required,min=6"`
	Role      string `json:"role"`
	Created   string `json:"-"`
	UpdatedAt string `json:"-"` // base do ETag (RFC 9111); não exposto no JSON do perfil
}
