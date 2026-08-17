//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"beer-review-app/internal/beer/model"
	userModel "beer-review-app/internal/user/model"
	"beer-review-app/pkg/uuid"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupPostgresContainer(ctx context.Context, t *testing.T) *sql.DB {
	t.Helper()

	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("beer_test_db"),
		postgres.WithUsername("test_user"),
		postgres.WithPassword("test_pass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("falha ao inicializar container postgres: %v", err)
	}

	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Errorf("falha ao encerrar container postgres: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("falha ao obter connection string do container: %v", err)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("falha ao abrir conexão com o banco postgres: %v", err)
	}

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("falha ao executar ping no banco postgres: %v", err)
	}

	runMigrations(ctx, t, db)

	return db
}

func runMigrations(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	schema := `
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

	CREATE TABLE IF NOT EXISTS beerUsers (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		username VARCHAR(50) NOT NULL UNIQUE,
		email VARCHAR(255) UNIQUE NOT NULL,
		password VARCHAR(255) NOT NULL,
		role VARCHAR(20) NOT NULL DEFAULT 'user',
		provider VARCHAR(20) NOT NULL DEFAULT 'local',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS beers (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(100) UNIQUE NOT NULL,
		style VARCHAR(50) NOT NULL,
		description TEXT,
		image_url TEXT,
		alcohol NUMERIC(4, 1) CHECK (alcohol >= 0 AND alcohol <= 70),
		taste VARCHAR(50) NOT NULL,
		aroma VARCHAR(50) NOT NULL,
		color VARCHAR(50) NOT NULL,
		body VARCHAR(50) NOT NULL,
		carbonation VARCHAR(50) NOT NULL,
		finish VARCHAR(50) NOT NULL,
		comments JSONB DEFAULT '[]'::jsonb,
		media JSONB DEFAULT '[]'::jsonb,
		created_by UUID NOT NULL REFERENCES beerUsers(id) ON DELETE CASCADE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS comments (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		beer_id UUID NOT NULL REFERENCES beers(id) ON DELETE CASCADE,
		user_id UUID NOT NULL REFERENCES beerUsers(id) ON DELETE CASCADE,
		text TEXT NOT NULL,
		rating INT CHECK (rating >= 1 AND rating <= 5),
		positive BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_beers_name ON beers(name);
	CREATE INDEX IF NOT EXISTS idx_beers_style ON beers(style);
	CREATE INDEX IF NOT EXISTS idx_beers_created_by ON beers(created_by);
	`

	if _, err := db.ExecContext(ctx, schema); err != nil {
		t.Fatalf("falha ao executar migrações de teste: %v", err)
	}
}

func createTestUser(ctx context.Context, t *testing.T, db *sql.DB, username, email, role string) string {
	t.Helper()
	user := userModel.User{
		ID:       uuid.MustNewV7(),
		Username: username,
		Email:    email,
		Password: "hashed_password",
		Role:     role,
		Created:  time.Now().UTC().Format(time.RFC3339),
	}
	err := db.QueryRowContext(ctx,
		"INSERT INTO beerUsers (id, username, email, password, role, created_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		user.ID, user.Username, user.Email, user.Password, user.Role, user.Created,
	).Scan(&user.ID)
	require.NoError(t, err)
	return user.ID
}

func TestIntegration_BeerRepository_CRUD_WithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := setupPostgresContainer(ctx, t)
	defer db.Close()

	repo, err := NewPostgresBeerRepository(db)
	require.NoError(t, err)

	ownerID := createTestUser(ctx, t, db, "owner", "owner@test.com", "user")

	beer := model.Beer{
		ID:          uuid.MustNewV7(),
		Name:        "IPA Teste",
		Style:       "IPA",
		Description: "Cerveja de teste",
		ImageUrl:    "https://example.com/img.jpg",
		Alcohol:     tcFloat64Ptr(5.5),
		Taste:       model.FlavorBitter,
		Aroma:       model.AromaCitrus,
		Color:       model.ColorAmber,
		Body:        model.BodyMedium,
		Carbonation: model.CarbonationMedium,
		Finish:      model.FinishDry,
		CreatedBy:   ownerID,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Comments:    []model.Comment{},
	}

	err = repo.Create(ctx, &beer)
	require.NoError(t, err)
	require.NotEmpty(t, beer.ID)

	got, err := repo.GetByID(ctx, beer.ID)
	require.NoError(t, err)
	require.Equal(t, "IPA Teste", got.Name)
	require.Equal(t, ownerID, got.CreatedBy)

	all, err := repo.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)

	page, total, err := repo.GetPaginated(ctx, 1, 10)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, page, 1)

	beer.Name = "IPA Atualizada"
	err = repo.Update(ctx, beer.ID, beer)
	require.NoError(t, err)

	got, _ = repo.GetByID(ctx, beer.ID)
	require.Equal(t, "IPA Atualizada", got.Name)

	err = repo.Delete(ctx, beer.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, beer.ID)
	require.Error(t, err)
}

func TestIntegration_BeerRepository_CascadeDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := setupPostgresContainer(ctx, t)
	defer db.Close()

	repo, err := NewPostgresBeerRepository(db)
	require.NoError(t, err)

	ownerID := createTestUser(ctx, t, db, "owner2", "owner2@test.com", "user")

	beer := model.Beer{
		ID:          uuid.MustNewV7(),
		Name:        "Stout Cascade",
		Style:       "Stout",
		Description: "Teste cascade",
		Alcohol:     tcFloat64Ptr(6.0),
		Taste:       model.FlavorBitter,
		Aroma:       model.AromaMalty,
		Color:       model.ColorDark,
		Body:        model.BodyFull,
		Carbonation: model.CarbonationLow,
		Finish:      model.FinishDry,
		CreatedBy:   ownerID,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Comments:    []model.Comment{},
	}
	err = repo.Create(ctx, &beer)
	require.NoError(t, err)

	comment := model.Comment{
		ID:        "comment-1",
		Text:      "Great beer!",
		Rating:    5,
		CreatedBy: ownerID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	err = repo.AddComment(ctx, beer.ID, comment)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, beer.ID)
	require.NoError(t, err)
	require.Len(t, got.Comments, 1)

	err = repo.Delete(ctx, beer.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, beer.ID)
	require.Error(t, err)

	var beerCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM beers WHERE id = $1", beer.ID).Scan(&beerCount)
	require.NoError(t, err)
	require.Equal(t, 0, beerCount)

	_, err = db.ExecContext(ctx, "DELETE FROM beerUsers WHERE id = $1", ownerID)
	require.NoError(t, err)

	var userCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM beerUsers WHERE id = $1", ownerID).Scan(&userCount)
	require.NoError(t, err)
	require.Equal(t, 0, userCount)
}

func TestIntegration_BeerRepository_DuplicateName_ReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := setupPostgresContainer(ctx, t)
	defer db.Close()

	repo, err := NewPostgresBeerRepository(db)
	require.NoError(t, err)

	ownerID := createTestUser(ctx, t, db, "owner3", "owner3@test.com", "user")

	beer1 := model.Beer{
		ID:          uuid.MustNewV7(),
		Name:        "Unique Beer",
		Style:       "IPA",
		Description: "First",
		Alcohol:     tcFloat64Ptr(5.0),
		Taste:       model.FlavorBitter,
		Aroma:       model.AromaCitrus,
		Color:       model.ColorAmber,
		Body:        model.BodyMedium,
		Carbonation: model.CarbonationMedium,
		Finish:      model.FinishDry,
		CreatedBy:   ownerID,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Comments:    []model.Comment{},
	}
	err = repo.Create(ctx, &beer1)
	require.NoError(t, err)

	beer2 := model.Beer{
		ID:          uuid.MustNewV7(),
		Name:        "Unique Beer",
		Style:       "IPA",
		Description: "Second",
		Alcohol:     tcFloat64Ptr(5.0),
		Taste:       model.FlavorBitter,
		Aroma:       model.AromaCitrus,
		Color:       model.ColorAmber,
		Body:        model.BodyMedium,
		Carbonation: model.CarbonationMedium,
		Finish:      model.FinishDry,
		CreatedBy:   ownerID,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Comments:    []model.Comment{},
	}
	err = repo.Create(ctx, &beer2)
	require.Error(t, err)
}

func TestIntegration_BeerRepository_SearchAndStats_WithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := setupPostgresContainer(ctx, t)
	defer db.Close()

	repo, err := NewPostgresBeerRepository(db)
	require.NoError(t, err)

	ownerID := createTestUser(ctx, t, db, "search_owner", "search@test.com", "user")

	beers := []model.Beer{
		{ID: uuid.MustNewV7(), Name: "IPA Search", Style: "IPA", CreatedBy: ownerID, CreatedAt: time.Now().UTC().Format(time.RFC3339), Alcohol: tcFloat64Ptr(5.5), Taste: model.FlavorBitter, Aroma: model.AromaCitrus, Color: model.ColorAmber, Body: model.BodyMedium, Carbonation: model.CarbonationMedium, Finish: model.FinishDry, Comments: []model.Comment{}},
		{ID: uuid.MustNewV7(), Name: "Stout Search", Style: "Stout", CreatedBy: ownerID, CreatedAt: time.Now().UTC().Format(time.RFC3339), Alcohol: tcFloat64Ptr(6.0), Taste: model.FlavorBitter, Aroma: model.AromaMalty, Color: model.ColorDark, Body: model.BodyFull, Carbonation: model.CarbonationLow, Finish: model.FinishDry, Comments: []model.Comment{}},
		{ID: uuid.MustNewV7(), Name: "IPA Nova", Style: "IPA", CreatedBy: ownerID, CreatedAt: time.Now().UTC().Format(time.RFC3339), Alcohol: tcFloat64Ptr(4.5), Taste: model.FlavorBitter, Aroma: model.AromaCitrus, Color: model.ColorAmber, Body: model.BodyMedium, Carbonation: model.CarbonationMedium, Finish: model.FinishDry, Comments: []model.Comment{}},
	}
	for _, b := range beers {
		err = repo.Create(ctx, &b)
		require.NoError(t, err)
	}

	results, total, err := repo.SearchBeers(ctx, model.BeerFilters{Query: "IPA", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, results, 2)

	results, total, err = repo.SearchBeers(ctx, model.BeerFilters{Style: "Stout", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, results, 1)
}

func TestIntegration_BeerRepository_RBAC_Ownership(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := setupPostgresContainer(ctx, t)
	defer db.Close()

	repo, err := NewPostgresBeerRepository(db)
	require.NoError(t, err)

	ownerID := createTestUser(ctx, t, db, "rbac_owner", "rbac_owner@test.com", "user")
	intruderID := createTestUser(ctx, t, db, "intruder", "intruder@test.com", "user")
	adminID := createTestUser(ctx, t, db, "admin", "admin@test.com", "admin")

	beer := model.Beer{
		ID:          uuid.MustNewV7(),
		Name:        "RBAC Beer",
		Style:       "IPA",
		Description: "Test RBAC",
		Alcohol:     tcFloat64Ptr(5.0),
		Taste:       model.FlavorBitter,
		Aroma:       model.AromaCitrus,
		Color:       model.ColorAmber,
		Body:        model.BodyMedium,
		Carbonation: model.CarbonationMedium,
		Finish:      model.FinishDry,
		CreatedBy:   ownerID,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Comments:    []model.Comment{},
	}
	err = repo.Create(ctx, &beer)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, beer.ID)
	require.NoError(t, err)

	beer.Name = "Hacked"
	err = repo.Update(ctx, beer.ID, beer)
	require.NoError(t, err)

	err = repo.Delete(ctx, beer.ID)
	require.NoError(t, err)

	beer2 := model.Beer{
		ID:          uuid.MustNewV7(),
		Name:        "RBAC Beer 2",
		Style:       "Stout",
		Description: "Test RBAC 2",
		Alcohol:     tcFloat64Ptr(6.0),
		Taste:       model.FlavorBitter,
		Aroma:       model.AromaMalty,
		Color:       model.ColorDark,
		Body:        model.BodyFull,
		Carbonation: model.CarbonationLow,
		Finish:      model.FinishDry,
		CreatedBy:   ownerID,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Comments:    []model.Comment{},
	}
	err = repo.Create(ctx, &beer2)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "UPDATE beers SET created_by = $1 WHERE id = $2", intruderID, beer2.ID)
	require.NoError(t, err)

	beer2.Name = "Intruder Update"
	err = repo.Update(ctx, beer2.ID, beer2)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "UPDATE beers SET created_by = $1 WHERE id = $2", adminID, beer2.ID)
	require.NoError(t, err)

	beer2.Name = "Admin Update"
	err = repo.Update(ctx, beer2.ID, beer2)
	require.NoError(t, err)

	err = repo.Delete(ctx, beer2.ID)
	require.NoError(t, err)
}

func tcFloat64Ptr(v float64) *float64 {
	return &v
}
