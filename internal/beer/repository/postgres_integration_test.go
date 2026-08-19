//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
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

	ALTER TABLE beers ADD COLUMN IF NOT EXISTS search_vector tsvector
	  GENERATED ALWAYS AS (
	    setweight(to_tsvector('portuguese', COALESCE(name, '')), 'A') ||
	    setweight(to_tsvector('portuguese', COALESCE(style, '')), 'B') ||
	    setweight(to_tsvector('portuguese', COALESCE(aroma, '')), 'C') ||
	    setweight(to_tsvector('portuguese', COALESCE(color, '')), 'C') ||
	    setweight(to_tsvector('portuguese', COALESCE(body, '')), 'C') ||
	    setweight(to_tsvector('portuguese', COALESCE(description, '')), 'D')
	  ) STORED;

	DROP INDEX IF EXISTS idx_beers_fts;
	CREATE INDEX idx_beers_fts ON beers USING GIN (search_vector);

	CREATE EXTENSION IF NOT EXISTS pg_trgm;
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

	results, total, _, err := repo.SearchBeers(ctx, model.BeerFilters{Query: "IPA", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, results, 2)

	results, total, _, err = repo.SearchBeers(ctx, model.BeerFilters{Style: "Stout", Page: 1, PageSize: 10})
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

// Seed100kBeers insere 100.000 beers de forma otimizada via COPY e ajusta
// parâmetros de memória do PostgreSQL para evitar gargalos de I/O durante a
// criação do índice GIN em testes de carga.
func Seed100kBeers(ctx context.Context, t *testing.T, db *sql.DB, userID string) int {
	t.Helper()
	t.Log("Configuring PostgreSQL memory for 100k beer seed...")
	_, _ = db.ExecContext(ctx, `
		SET shared_buffers = '256MB';
		SET maintenance_work_mem = '256MB';
		SET work_mem = '64MB';
		SET random_page_cost = 1.0;
	`)

	t.Log("Creating temporary table for COPY...")
	_, err := db.ExecContext(ctx, `
		CREATE TEMP TABLE tmp_beer_seed (
			name VARCHAR(100),
			style VARCHAR(50),
			description TEXT,
			alcohol NUMERIC(4,1),
			taste VARCHAR(50),
			aroma VARCHAR(50),
			color VARCHAR(50),
			body VARCHAR(50),
			carbonation VARCHAR(50),
			finish VARCHAR(50),
			created_by UUID
		) ON COMMIT DROP;
	`)
	require.NoError(t, err)

	t.Log("Building 100k rows in memory...")
	rows := make([][]interface{}, 0, 100000)
	styles := []string{"IPA", "Stout", "Pilsner", "Wheat", "Porter", "Ale", "Lager", "Sour", "Barleywine", "Saison"}
	tastes := []string{"Doce", "Amargo", "Equilibrado", "Frutado", "Floral", "Malte", "Cítrico", "Herbal"}
	aromas := []string{"Cítrico", "Floral", "Malte", "Herbal", "Frutado", "Caramelo", "Torrado"}
	colors := []string{"Clara", "Âmbar", "Escura", "Rubi", "Dourada"}
	bodies := []string{"Leve", "Médio", "Corpo", "Intenso"}
	carbonations := []string{"Baixa", "Média", "Alta"}
	finishes := []string{"Seco", "Doce", "Amargo", "Suave", "Cremoso"}

	for i := 0; i < 100000; i++ {
		style := styles[i%len(styles)]
		rows = append(rows, []interface{}{
			fmt.Sprintf("Beer %d - %s", i, style),
			style,
			fmt.Sprintf("Description for beer %d", i),
			randFloat(4, 1),
			tastes[i%len(tastes)],
			aromas[i%len(aromas)],
			colors[i%len(colors)],
			bodies[i%len(bodies)],
			carbonations[i%len(carbonations)],
			finishes[i%len(finishes)],
			userID,
		})
	}

	t.Log("COPYing 100k rows into temp table...")
	copyCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	tx, err := db.BeginTx(copyCtx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(copyCtx, pq.CopyIn("tmp_beer_seed",
		"name", "style", "description", "alcohol", "taste", "aroma", "color", "body", "carbonation", "finish", "created_by",
	))
	require.NoError(t, err)

	for _, row := range rows {
		_, err = stmt.ExecContext(copyCtx, row...)
		require.NoError(t, err)
	}
	_, err = stmt.ExecContext(copyCtx)
	require.NoError(t, err)
	require.NoError(t, stmt.Close())

	require.NoError(t, tx.Commit())

	t.Log("Inserting from temp table with tsvector...")
	_, err = db.ExecContext(ctx, `
		INSERT INTO beers (name, style, description, alcohol, taste, aroma, color, body, carbonation, finish, created_by, created_at, updated_at)
		SELECT name, style, description, alcohol, taste, aroma, color, body, carbonation, finish, created_by, NOW(), NOW()
		FROM tmp_beer_seed
	`)
	require.NoError(t, err)

	t.Log("Creating GIN index on 100k beers...")
	_, err = db.ExecContext(ctx, `
		DROP INDEX IF EXISTS idx_beers_fts;
		CREATE INDEX CONCURRENTLY idx_beers_fts ON beers USING GIN (search_vector);
	`)
	require.NoError(t, err)

	count := 0
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM beers").Scan(&count))
	t.Logf("Seeded %d beers", count)
	return count
}

func randFloat(max int, prec int) float64 {
	return float64(rand.Intn(max*10)) / float64(10)
}
