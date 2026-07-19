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
	// Para contas OAuth (provider != "local") o Password vai vazio/NULL.
	Password    string `json:"password,omitempty" validate:"omitempty,min=6"`
	Role        string `json:"role"`
	Provider    string `json:"-"` // "local" | "google" | "apple"; não exposto
	ExternalSub string `json:"-"` // sub do IdP OIDC; não exposto
	Created     string `json:"-"`
	UpdatedAt   string `json:"-"` // base do ETag (RFC 9111); não exposto no JSON do perfil
}

// Constantes de provedor de identidade para login social (RFC 6749 / OIDC).
const (
	ProviderLocal  = "local"
	ProviderGoogle = "google"
	ProviderApple  = "apple"
)
