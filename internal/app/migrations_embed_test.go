package app

import (
	"strings"
	"testing"
)

// TestEmbeddedMigrationsPresent valida que todos os ficheiros de migracao
// listados em migrateDB estao efetivamente embutidos (go:embed) e leveis em
// runtime.
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

// TestResetSchemaEnabled é table-driven e valida a flag DB_RESET_SCHEMA que
// controla o reset destrutivo do esquema (banco como fonte de verdade).
// Default false; o reset destrutivo precisa ser explicitamente habilitado.
func TestResetSchemaEnabled(t *testing.T) {
	tests := []struct {
		name string
		env  string // valor de DB_RESET_SCHEMA (vazio = default)
		want bool
	}{
		{"default (vazio)", "", false},
		{"true", "true", true},
		{"1", "1", true},
		{"false", "false", false},
		{"0", "0", false},
		{"no", "no", false},
		{"FALSE maiusculo", "FALSE", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DB_RESET_SCHEMA", tt.env)
			if got := resetSchemaEnabled(); got != tt.want {
				t.Fatalf("resetSchemaEnabled() com DB_RESET_SCHEMA=%q = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

// TestGuardDestructiveResetInProduction verifica que a combinação letal
// ENV=production + DB_RESET_SCHEMA=true é bloqueada com panic antes de
// qualquer operação destrutiva no banco. É a trava de segurança que
// protege dados reais contra cold starts em produção.
func TestGuardDestructiveResetInProduction(t *testing.T) {
	cases := []struct {
		name      string
		env       string
		resetEnv  string
		wantPanic bool
	}{
		{"production + reset=true PANIC", "production", "true", true},
		{"prod + reset=true PANIC", "prod", "true", true},
		{"PRODUCTION + reset=true PANIC (case-insensitive)", "PRODUCTION", "true", true},
		{"production + reset=false OK", "production", "false", false},
		{"production + reset=0 OK", "production", "0", false},
		{"dev + reset=true OK", "dev", "true", false},
		{"env vazio + reset=true OK", "", "true", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENV", tc.env)
			t.Setenv("DB_RESET_SCHEMA", tc.resetEnv)
			defer func() {
				if r := recover(); r != nil {
					if !tc.wantPanic {
						t.Fatalf("panic inesperado: %v", r)
					}
				} else if tc.wantPanic {
					t.Fatal("esperava panic, mas não houve")
				}
			}()
			guardDestructiveResetInProduction()
		})
	}
}
