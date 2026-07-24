package model

// PushSubscription representa uma subscrição de Web Push (RFC 8030) para um utilizador.
type PushSubscription struct {
	ID         string `json:"id"`
	UserID     string `json:"userId"`
	Endpoint   string `json:"endpoint"`
	P256DH     string `json:"p256dh"`
	Auth       string `json:"auth"`
	UserAgent  string `json:"userAgent,omitempty"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
}
