package repository

import (
	"context"
	"database/sql"
	stdErr "errors"
	"net/http"
	"testing"
	"time"

	"beer-review-app/internal/user/model"
	"beer-review-app/pkg/errors"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresUserRepository(t *testing.T) {
	tests := []struct {
		name    string
		db      *sql.DB
		wantErr bool
		errCode int
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
			repo, err := NewPostgresUserRepository(tt.db)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewPostgresUserRepository() error = %v, wantErr %v", err, tt.wantErr)
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

func TestPostgresUserRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name    string
		user    model.User
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			user: model.User{ID: "u1", Username: "alice", Email: "alice@example.com", Role: model.RoleUser, Created: time.Now().Format(time.RFC3339)},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO beerUsers").
					WithArgs("u1", "alice", "alice@example.com", sqlmock.AnyArg(), model.RoleUser, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:    "exec error",
			user:    model.User{ID: "u1", Username: "alice", Email: "alice@example.com", Role: model.RoleUser, Created: time.Now().Format(time.RFC3339)},
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO beerUsers").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			err := repo.Create(context.Background(), tt.user)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresUserRepository_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name     string
		id       string
		setup    func(sqlmock.Sqlmock)
		wantErr  bool
		errCode  int
		wantUser string
	}{
		{
			name:     "success",
			id:       "u1",
			wantUser: "alice",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("u1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "role", "created", "updated_at"}).
						AddRow("u1", "alice", "alice@example.com", model.RoleUser, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339)))
			},
		},
		{
			name:     "not found",
			id:       "999",
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("999").
					WillReturnError(sql.ErrNoRows)
			},
		},
		{
			name:     "query error",
			id:       "u1",
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("u1").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			user, err := repo.GetByID(context.Background(), tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetByID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			require.Equal(t, tt.wantUser, user.Username)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresUserRepository_GetByEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name     string
		email    string
		setup    func(sqlmock.Sqlmock)
		wantErr  bool
		wantUser string
	}{
		{
			name:     "success",
			email:    "alice@example.com",
			wantUser: "alice",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("alice@example.com").
					WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password", "role", "created", "updated_at"}).
						AddRow("u1", "alice", "alice@example.com", "hash", model.RoleUser, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339)))
			},
		},
		{
			name:    "not found",
			email:   "missing@example.com",
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("missing@example.com").
					WillReturnError(sql.ErrNoRows)
			},
		},
		{
			name:    "query error",
			email:   "alice@example.com",
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("alice@example.com").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			user, err := repo.GetByEmail(context.Background(), tt.email)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetByEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			require.Equal(t, tt.wantUser, user.Username)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresUserRepository_GetByExternal(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name     string
		provider string
		sub      string
		setup    func(sqlmock.Sqlmock)
		wantErr  bool
		wantUser string
	}{
		{
			name:     "success",
			provider: "google",
			sub:      "sub-123",
			wantUser: "bob",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("google", "sub-123").
					WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "role", "provider", "external_sub", "created", "updated_at"}).
						AddRow("u2", "bob", "bob@example.com", model.RoleUser, "google", "sub-123", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339)))
			},
		},
		{
			name:    "not found",
			provider: "google",
			sub:     "missing",
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("google", "missing").
					WillReturnError(sql.ErrNoRows)
			},
		},
		{
			name:    "query error",
			provider: "google",
			sub:     "sub-123",
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("google", "sub-123").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			user, err := repo.GetByExternal(context.Background(), tt.provider, tt.sub)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetByExternal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			require.Equal(t, tt.wantUser, user.Username)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresUserRepository_UpsertByExternal(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name    string
		user    model.User
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success insert",
			user: model.User{Username: "new", Email: "new@example.com", Role: model.RoleUser, Provider: "google", ExternalSub: "sub-new", Created: time.Now().Format(time.RFC3339)},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("INSERT").
					WithArgs(sqlmock.AnyArg(), "new", "new@example.com", model.RoleUser, "google", "sub-new", sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "role", "provider", "external_sub", "created", "updated_at"}).
						AddRow("u3", "new", "new@example.com", model.RoleUser, "google", "sub-new", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339)))
			},
		},
		{
			name: "success update",
			user: model.User{Username: "bob2", Email: "bob2@example.com", Role: model.RoleUser, Provider: "google", ExternalSub: "sub-123", Created: time.Now().Format(time.RFC3339)},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("INSERT").
					WithArgs(sqlmock.AnyArg(), "bob2", "bob2@example.com", model.RoleUser, "google", "sub-123", sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "role", "provider", "external_sub", "created", "updated_at"}).
						AddRow("u2", "bob2", "bob2@example.com", model.RoleUser, "google", "sub-123", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339)))
			},
		},
		{
			name:    "query error",
			user:    model.User{Username: "new", Email: "new@example.com", Role: model.RoleUser, Provider: "google", ExternalSub: "sub-new", Created: time.Now().Format(time.RFC3339)},
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("INSERT").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			user, err := repo.UpsertByExternal(context.Background(), tt.user)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UpsertByExternal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				require.Equal(t, tt.user.Username, user.Username)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresUserRepository_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name    string
		id      string
		user    model.User
		setup   func(sqlmock.Sqlmock)
		wantErr bool
		errCode int
	}{
		{
			name: "success",
			id:   "u1",
			user: model.User{Username: "alice2", Email: "alice@example.com", Role: model.RoleUser},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE beerUsers").
					WithArgs("alice2", "alice@example.com", "u1").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:    "exec error",
			id:      "u1",
			user:    model.User{Username: "alice2", Email: "alice@example.com", Role: model.RoleUser},
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE beerUsers").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
		{
			name:     "rows affected 0",
			id:       "999",
			user:     model.User{Username: "alice2", Email: "alice@example.com", Role: model.RoleUser},
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE beerUsers").
					WithArgs("alice2", "alice@example.com", "999").
					WillReturnResult(sqlmock.NewResult(1, 0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			err := repo.Update(context.Background(), tt.id, tt.user)
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

func TestPostgresUserRepository_Delete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name    string
		id      string
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   "u1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM beerUsers").
					WithArgs("u1").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:    "exec error",
			id:      "u1",
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM beerUsers").
					WithArgs("u1").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
		{
			name:     "rows affected 0",
			id:       "999",
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM beerUsers").
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
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresUserRepository_List(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name     string
		page     int
		pageSize int
		setup    func(sqlmock.Sqlmock)
		wantErr  bool
		wantLen  int
		wantTotal int
	}{
		{
			name:     "success",
			page:     1,
			pageSize: 10,
			wantLen:  1,
			wantTotal: 1,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				m.ExpectQuery("SELECT").
					WithArgs(10, 0).
					WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "role", "created"}).
						AddRow("u1", "alice", "alice@example.com", model.RoleUser, time.Now().Format(time.RFC3339)))
			},
		},
		{
			name:     "count error",
			page:     1,
			pageSize: 10,
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT COUNT").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
		{
			name:     "query error",
			page:     1,
			pageSize: 10,
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				m.ExpectQuery("SELECT").
					WithArgs(10, 0).
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			users, total, err := repo.List(context.Background(), tt.page, tt.pageSize)
			if (err != nil) != tt.wantErr {
				t.Fatalf("List() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				require.Len(t, users, tt.wantLen)
				require.Equal(t, tt.wantTotal, total)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresUserRepository_CreatePushSubscription(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name    string
		sub     model.PushSubscription
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			sub:  model.PushSubscription{UserID: "u1", Endpoint: "https://push.example.com", P256DH: "key1", Auth: "auth1", UserAgent: "Mozilla"},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO push_subscriptions").
					WithArgs("u1", "https://push.example.com", "key1", "auth1", "Mozilla").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:    "exec error",
			sub:     model.PushSubscription{UserID: "u1", Endpoint: "https://push.example.com", P256DH: "key1", Auth: "auth1"},
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO push_subscriptions").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			err := repo.CreatePushSubscription(context.Background(), tt.sub)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreatePushSubscription() error = %v, wantErr %v", err, tt.wantErr)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresUserRepository_ListPushSubscriptions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name    string
		userID  string
		setup   func(sqlmock.Sqlmock)
		wantErr bool
		wantLen int
	}{
		{
			name:    "success",
			userID:  "u1",
			wantLen: 1,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("u1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "endpoint", "p256dh", "auth", "user_agent", "created_at", "updated_at", "last_used_at"}).
						AddRow("s1", "u1", "https://push.example.com", "key1", "auth1", "Mozilla", time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339), ""))
			},
		},
		{
			name:    "query error",
			userID:  "u1",
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("u1").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			subs, err := repo.ListPushSubscriptions(context.Background(), tt.userID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ListPushSubscriptions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				require.Len(t, subs, tt.wantLen)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresUserRepository_DeletePushSubscription(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name    string
		id      string
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   "s1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM push_subscriptions").
					WithArgs("s1").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:    "exec error",
			id:      "s1",
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM push_subscriptions").
					WithArgs("s1").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
		{
			name:     "rows affected 0",
			id:       "missing",
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM push_subscriptions").
					WithArgs("missing").
					WillReturnResult(sqlmock.NewResult(1, 0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			err := repo.DeletePushSubscription(context.Background(), tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DeletePushSubscription() error = %v, wantErr %v", err, tt.wantErr)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresUserRepository_DeletePushSubscriptionByEndpoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name     string
		userID   string
		endpoint string
		setup    func(sqlmock.Sqlmock)
		wantErr  bool
	}{
		{
			name:     "success",
			userID:   "u1",
			endpoint: "https://push.example.com",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM push_subscriptions").
					WithArgs("u1", "https://push.example.com").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:     "exec error",
			userID:   "u1",
			endpoint: "https://push.example.com",
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM push_subscriptions").
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
		{
			name:     "rows affected 0",
			userID:   "u1",
			endpoint: "https://missing.example.com",
			wantErr:  true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM push_subscriptions").
					WithArgs("u1", "https://missing.example.com").
					WillReturnResult(sqlmock.NewResult(1, 0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			err := repo.DeletePushSubscriptionByEndpoint(context.Background(), tt.userID, tt.endpoint)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DeletePushSubscriptionByEndpoint() error = %v, wantErr %v", err, tt.wantErr)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPostgresUserRepository_GetMemberSince(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &PostgresUserRepository{db: db}

	tests := []struct {
		name    string
		userID  string
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name:   "success",
			userID: "u1",
			setup: func(m sqlmock.Sqlmock) {
				created := time.Now().AddDate(0, 0, -10)
				m.ExpectQuery("SELECT").
					WithArgs("u1").
					WillReturnRows(sqlmock.NewRows([]string{"created"}).
						AddRow(created))
			},
		},
		{
			name:    "not found",
			userID:  "missing",
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("missing").
					WillReturnError(sql.ErrNoRows)
			},
		},
		{
			name:    "query error",
			userID:  "u1",
			wantErr: true,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT").
					WithArgs("u1").
					WillReturnError(http.ErrHandlerTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(mock)
			_, err := repo.GetMemberSince(context.Background(), tt.userID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetMemberSince() error = %v, wantErr %v", err, tt.wantErr)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
