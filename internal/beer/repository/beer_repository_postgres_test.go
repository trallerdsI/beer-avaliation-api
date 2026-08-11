package repository

import (
	"context"
	"database/sql"
	stdErr "errors"
	"net/http"
	"testing"
	"time"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresBeerRepository(t *testing.T) {
	tests := []struct {
		name     string
		db       *sql.DB
		wantErr  bool
		errCode  int
	}{
		{
			name:    "nil db returns unavailable error",
			db:      nil,
			wantErr: true,
			errCode: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, err := NewPostgresBeerRepository(tt.db)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewPostgresBeerRepository() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var appErr *errors.AppError
				if !stdErr.As(err, &appErr) {
					t.Fatalf("expected *AppError, got %T", err)
				}
				if appErr.Code != tt.errCode {
					t.Fatalf("error code = %d, want %d", appErr.Code, tt.errCode)
				}
				return
			}
			require.NotNil(t, repo)
		})
	}
}

func TestPostgresBeerRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresBeerRepository{db: db}

	tests := []struct {
		name      string
		beer      *model.Beer
		setup     func(sqlmock.Sqlmock)
		wantErr   bool
		errCode   int
	}{
		{
			name: "success",
			beer: &model.Beer{ID: "1", Name: "IPA", CreatedBy: "u1", CreatedAt: time.Now().Format(time.RFC3339)},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO beers").
					WithArgs("1", "IPA", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "u1", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name: "db exec error",
			beer: &model.Beer{ID: "2", Name: "IPA", CreatedBy: "u1", CreatedAt: time.Now().Format(time.RFC3339)},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO beers").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(http.ErrHandlerTimeout)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			err := repo.Create(context.Background(), tt.beer)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr && tt.errCode > 0 {
				var appErr *errors.AppError
				if !stdErr.As(err, &appErr) {
					t.Fatalf("expected *AppError, got %T", err)
				}
				if appErr.Code != tt.errCode {
					t.Fatalf("error code = %d, want %d", appErr.Code, tt.errCode)
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresBeerRepository_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresBeerRepository{db: db}

	tests := []struct {
		name     string
		id       string
		setup    func(sqlmock.Sqlmock)
		wantErr  bool
		errCode  int
		wantName string
	}{
		{
			name:     "success",
			id:       "1",
			wantName: "IPA",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "style", "description", "image_url", "alcohol", "taste", "aroma", "color", "body", "carbonation", "finish", "comments", "created_by", "created_at", "updated_at", "media"}).
						AddRow("1", "IPA", "Ale", "desc", "img.jpg", 5.5, "Doce", "Floral", "Clara", "Leve", "Baixa", "Seco", []byte("[]"), "u1", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339), []byte("[]")))
			},
		},
		{
			name:     "not found",
			id:       "999",
			wantErr:  true,
			errCode:  http.StatusNotFound,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("999").
					WillReturnError(sql.ErrNoRows)
			},
		},
		{
			name:     "query error",
			id:       "1",
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("1").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			beer, err := repo.GetByID(context.Background(), tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetByID() error = %v, wantErr %v", err, tt.wantErr)
			}
		if tt.wantErr {
			if tt.errCode > 0 {
				var appErr *errors.AppError
				if !stdErr.As(err, &appErr) {
					t.Fatalf("expected *AppError, got %T", err)
				}
				if appErr.Code != tt.errCode {
					t.Fatalf("error code = %d, want %d", appErr.Code, tt.errCode)
				}
			}
			return
		}
			require.Equal(t, tt.wantName, beer.Name)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresBeerRepository_GetPaginated(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresBeerRepository{db: db}

	tests := []struct {
		name      string
		page      int
		pageSize  int
		setup     func(sqlmock.Sqlmock)
		wantErr   bool
		wantBeers int
		wantTotal int
	}{
		{
			name:      "success",
			page:      1,
			pageSize:  10,
			wantBeers: 1,
			wantTotal: 1,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs(10, 0).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "style", "description", "image_url", "alcohol", "taste", "aroma", "color", "body", "carbonation", "finish", "comments", "created_by", "created_at", "updated_at", "media"}).
						AddRow("1", "IPA", "Ale", "desc", "img.jpg", 5.5, "Doce", "Floral", "Clara", "Leve", "Baixa", "Seco", []byte("[]"), "u1", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339), []byte("[]")))
				m.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			},
		},
		{
			name:     "query error",
			page:     1,
			pageSize: 10,
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs(10, 0).
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
		{
			name:     "count query error",
			page:     1,
			pageSize: 10,
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs(10, 0).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "style", "description", "image_url", "alcohol", "taste", "aroma", "color", "body", "carbonation", "finish", "comments", "created_by", "created_at", "updated_at", "media"}).
						AddRow("1", "IPA", "Ale", "desc", "img.jpg", 5.5, "Doce", "Floral", "Clara", "Leve", "Baixa", "Seco", []byte("[]"), "u1", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339), []byte("[]")))
				m.ExpectQuery("SELECT COUNT").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			beers, total, err := repo.GetPaginated(context.Background(), tt.page, tt.pageSize)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetPaginated() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				require.Len(t, beers, tt.wantBeers)
				require.Equal(t, tt.wantTotal, total)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresBeerRepository_GetAll(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresBeerRepository{db: db}

	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr bool
		wantLen int
	}{
		{
			name:    "success",
			wantLen: 1,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "style", "description", "image_url", "alcohol", "taste", "aroma", "color", "body", "carbonation", "finish", "comments", "created_by", "created_at", "updated_at", "media"}).
						AddRow("1", "IPA", "Ale", "desc", "img.jpg", 5.5, "Doce", "Floral", "Clara", "Leve", "Baixa", "Seco", []byte("[]"), "u1", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339), []byte("[]")))
			},
		},
		{
			name:    "query error",
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			beers, err := repo.GetAll(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetAll() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				require.Len(t, beers, tt.wantLen)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresBeerRepository_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresBeerRepository{db: db}

	tests := []struct {
		name     string
		id       string
		beer     model.Beer
		setup    func(sqlmock.Sqlmock)
		wantErr  bool
		errCode  int
	}{
		{
			name: "success",
			id:   "1",
			beer: model.Beer{Name: "IPA", Style: "Ale", Description: "desc", ImageUrl: "img.jpg", Alcohol: float64Ptr(5.5), Taste: "Doce", Aroma: "Floral", Color: "Clara", Body: "Leve", Carbonation: "Baixa", Finish: "Seco", CreatedBy: "u1", CreatedAt: time.Now().Format(time.RFC3339)},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE beers").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "1").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:    "exec error",
			id:      "1",
			beer:    model.Beer{Name: "IPA", CreatedBy: "u1", CreatedAt: time.Now().Format(time.RFC3339)},
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE beers").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "1").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
		{
			name:     "rows affected 0",
			id:       "999",
			beer:     model.Beer{Name: "IPA", CreatedBy: "u1", CreatedAt: time.Now().Format(time.RFC3339)},
			wantErr:  true,
			errCode:  http.StatusNotFound,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE beers").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "999").
					WillReturnResult(sqlmock.NewResult(1, 0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			err := repo.Update(context.Background(), tt.id, tt.beer)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Update() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errCode > 0 {
				var appErr *errors.AppError
				if !stdErr.As(err, &appErr) {
					t.Fatalf("expected *AppError, got %T", err)
				}
				if appErr.Code != tt.errCode {
					t.Fatalf("error code = %d, want %d", appErr.Code, tt.errCode)
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresBeerRepository_Delete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresBeerRepository{db: db}

	tests := []struct {
		name     string
		id       string
		setup    func(sqlmock.Sqlmock)
		wantErr  bool
		errCode  int
	}{
		{
			name: "success",
			id:   "1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM beers").
					WithArgs("1").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:    "exec error",
			id:      "1",
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM beers").
					WithArgs("1").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
		{
			name:     "rows affected 0",
			id:       "999",
			wantErr:  true,
			errCode:  http.StatusNotFound,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM beers").
					WithArgs("999").
					WillReturnResult(sqlmock.NewResult(1, 0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			err := repo.Delete(context.Background(), tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errCode > 0 {
				var appErr *errors.AppError
				if !stdErr.As(err, &appErr) {
					t.Fatalf("expected *AppError, got %T", err)
				}
				if appErr.Code != tt.errCode {
					t.Fatalf("error code = %d, want %d", appErr.Code, tt.errCode)
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresBeerRepository_AddComment(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresBeerRepository{db: db}

	tests := []struct {
		name     string
		id       string
		comment  model.Comment
		setup    func(sqlmock.Sqlmock)
		wantErr  bool
		errCode  int
	}{
		{
			name:    "success",
			id:      "1",
			comment: model.Comment{ID: "c1", Text: "Great", Rating: 5},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "style", "description", "image_url", "alcohol", "taste", "aroma", "color", "body", "carbonation", "finish", "comments", "created_by", "created_at", "updated_at", "media"}).
						AddRow("1", "IPA", "Ale", "desc", "img.jpg", 5.5, "Doce", "Floral", "Clara", "Leve", "Baixa", "Seco", []byte("[]"), "u1", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339), []byte("[]")))
				m.ExpectExec("UPDATE beers").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "1").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:     "beer not found",
			id:       "999",
			comment:  model.Comment{ID: "c1", Text: "Great", Rating: 5},
			wantErr:  true,
			errCode:  http.StatusNotFound,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("999").
					WillReturnError(sql.ErrNoRows)
			},
		},
		{
			name:    "update error after get",
			id:      "1",
			comment: model.Comment{ID: "c1", Text: "Great", Rating: 5},
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "style", "description", "image_url", "alcohol", "taste", "aroma", "color", "body", "carbonation", "finish", "comments", "created_by", "created_at", "updated_at", "media"}).
						AddRow("1", "IPA", "Ale", "desc", "img.jpg", 5.5, "Doce", "Floral", "Clara", "Leve", "Baixa", "Seco", []byte("[]"), "u1", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339), []byte("[]")))
				m.ExpectExec("UPDATE beers").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "1").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			err := repo.AddComment(context.Background(), tt.id, tt.comment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AddComment() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errCode > 0 {
				var appErr *errors.AppError
				if !stdErr.As(err, &appErr) {
					t.Fatalf("expected *AppError, got %T", err)
				}
				if appErr.Code != tt.errCode {
					t.Fatalf("error code = %d, want %d", appErr.Code, tt.errCode)
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresBeerRepository_DeleteComment(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresBeerRepository{db: db}

	tests := []struct {
		name      string
		beerID    string
		commentID string
		setup     func(sqlmock.Sqlmock)
		wantErr   bool
		errCode   int
	}{
		{
			name:      "success",
			beerID:    "1",
			commentID: "c1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "style", "description", "image_url", "alcohol", "taste", "aroma", "color", "body", "carbonation", "finish", "comments", "created_by", "created_at", "updated_at", "media"}).
						AddRow("1", "IPA", "Ale", "desc", "img.jpg", 5.5, "Doce", "Floral", "Clara", "Leve", "Baixa", "Seco", []byte("[{\"id\":\"c1\",\"text\":\"Great\",\"rating\":5}]"), "u1", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339), []byte("[]")))
				m.ExpectExec("UPDATE beers").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "1").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:      "beer not found",
			beerID:    "999",
			commentID: "c1",
			wantErr:   true,
			errCode:   http.StatusNotFound,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("999").
					WillReturnError(sql.ErrNoRows)
			},
		},
		{
			name:      "comment not found",
			beerID:    "1",
			commentID: "c2",
			wantErr:   true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "style", "description", "image_url", "alcohol", "taste", "aroma", "color", "body", "carbonation", "finish", "comments", "created_by", "created_at", "updated_at", "media"}).
						AddRow("1", "IPA", "Ale", "desc", "img.jpg", 5.5, "Doce", "Floral", "Clara", "Leve", "Baixa", "Seco", []byte("[{\"id\":\"c1\",\"text\":\"Great\",\"rating\":5}]"), "u1", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339), []byte("[]")))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			err := repo.DeleteComment(context.Background(), tt.beerID, tt.commentID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DeleteComment() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errCode > 0 {
				var appErr *errors.AppError
				if !stdErr.As(err, &appErr) {
					t.Fatalf("expected *AppError, got %T", err)
				}
				if appErr.Code != tt.errCode {
					t.Fatalf("error code = %d, want %d", appErr.Code, tt.errCode)
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}
