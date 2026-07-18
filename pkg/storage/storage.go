// Package storage implementa o cliente de object storage usado para ingesta de
// mídia do app Flutter (RFC 7578). O backend é o Supabase Storage, mas a
// interface Uploader mantém o usecase desacoplado (Clean Architecture) e
// permite mocking em testes. A implementação usa apenas net/http contra a REST
// API do Supabase Storage — sem dependência externa (Pilar Zero-Dependency).
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Uploader é a porta de saída para armazenamento de objetos. O usecase depende
// apenas desta interface, nunca do cliente concreto.
type Uploader interface {
	// Upload guarda data sob o nome dado e devolve a URL pública/assinada.
	Upload(ctx context.Context, objectName, contentType string, data []byte) (string, error)
}

// SupabaseStorage é um Uploader para o Supabase Storage (REST API).
// Documentação: https://supabase.com/docs/guides/storage/api
type SupabaseStorage struct {
	baseURL    string // ex: https://<project>.supabase.co
	bucket     string
	serviceKey string // service_role key (server-side, NUNCA exposta ao cliente)
	client     *http.Client
}

// NewSupabaseStorageFromEnv constrói o cliente a partir de variáveis de ambiente.
// Sem fallback inseguro: se faltarem, devolve erro (o upload simplesmente não
// está disponível, mas o servidor não usa credencial hardcoded).
func NewSupabaseStorageFromEnv() (*SupabaseStorage, error) {
	base := os.Getenv("SUPABASE_URL")
	bucket := os.Getenv("SUPABASE_STORAGE_BUCKET")
	key := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if base == "" || bucket == "" || key == "" {
		return nil, fmt.Errorf("storage não configurado: definir SUPABASE_URL, SUPABASE_STORAGE_BUCKET e SUPABASE_SERVICE_ROLE_KEY")
	}
	return &SupabaseStorage{
		baseURL:    strings.TrimRight(base, "/"),
		bucket:     bucket,
		serviceKey: key,
		client:     &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Upload faz PUT em /storage/v1/object/{bucket}/{name} com a service-role key.
// Devolve a URL pública do objeto (o bucket deve estar configurado como public
// ou o cliente deve gerar URL assinada à parte).
func (s *SupabaseStorage) Upload(ctx context.Context, objectName, contentType string, data []byte) (string, error) {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.baseURL, s.bucket, objectName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("storage: falha ao montar request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceKey)
	req.Header.Set("apikey", s.serviceKey)
	req.Header.Set("Content-Type", contentType)
	// Upsert permite re-upload da mesma cerveja sem erro de "já existe".
	req.Header.Set("x-upsert", "true")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("storage: falha ao enviar objeto: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("storage: status %d: %s", resp.StatusCode, string(body))
	}

	return s.publicURL(objectName), nil
}

// publicURL devolve a URL pública do objeto (assume bucket public).
func (s *SupabaseStorage) publicURL(objectName string) string {
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.baseURL, s.bucket, objectName)
}
