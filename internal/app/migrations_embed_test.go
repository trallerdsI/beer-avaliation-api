package app

import (
	"strings"
	"testing"
)

// TestEmbeddedMigrationsPresent valida que todos os ficheiros de migracao
// listados em migrateDB estao efetivamente embutidos (go:embed) e leveis em
// runtime. Isto evita o erro de 'migration file not found' que ocorria na
// Vercel quando o embed apontava para internal/app/migrations/ (correto) mas
// os ficheiros estavam na raiz migrations/ (fora do alcance do embed).
func TestEmbeddedMigrationsPresent(t *testing.T) {
	want := []string{
		"create_users_table.sql",
		"add_role_to_users.sql",
		"create_beers_table.sql",
		"add_created_by_to_beers.sql",
		"add_created_at_to_beers.sql",
		"add_comments_jsonb.sql",
		"create_indexes.sql",
		"use_uuid_pk.sql",
		"add_updated_at.sql",
		"extend_media.sql",
		"drop_legacy_comments_table.sql",
	}
	for _, name := range want {
		data, err := readMigrationSQL("migrations/" + name)
		if err != nil {
			t.Fatalf("migração %s não encontrada no embed: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("migração %s parece vazia", name)
		}
		up := strings.ToUpper(string(data))
		if !strings.Contains(up, "CREATE") && !strings.Contains(up, "ALTER") && !strings.Contains(up, "DROP") {
			t.Fatalf("migração %s parece inválida (sem CREATE/ALTER/DROP)", name)
		}
	}
}
