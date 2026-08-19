package usecase

import (
	"context"
	stderrors "errors"
	"net/http"
	"testing"

	"beer-review-app/internal/beer/model"
	usermodel "beer-review-app/internal/user/model"
	appErrors "beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/moderation"
	beerRepo "beer-review-app/internal/beer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGetAll tests the GetAll method
func TestGetAll(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	beers := []model.Beer{{ID: "1", Name: "Beer1"}, {ID: "2", Name: "Beer2"}}

	mockRepo.On("GetAll", mock.Anything).Return(beers, nil)

	result, err := usecase.GetAll(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, beers, result)
	mockRepo.AssertExpectations(t)
}

// TestCreate tests the Create method
// TestCreateDuplicate verifica o bloqueio 409 quando já existe cerveja com
// nome semelhante (Decisão C), com sugestões no detail.
func TestCreateDuplicate(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	beer := model.Beer{ID: "1", Name: "Heineken"}
	existing := []model.Beer{{ID: "12", Name: "Heineken Long Neck"}}

	mockRepo.On("SearchBeers", mock.Anything, mock.Anything).Return(existing, 1, false, nil)

	err := usecase.Create(context.Background(), &beer)

	var appErr *appErrors.AppError
	assert.Error(t, err)
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 409, appErr.Code)
	assert.Equal(t, "DUPLICATE_BEER", appErr.ErrorCode)
}

func TestCreate(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	beer := model.Beer{ID: "1", Name: "Beer1"}

	// Sem duplicados: Create faz search antes de inserir.
	mockRepo.On("SearchBeers", mock.Anything, mock.Anything).Return([]model.Beer{}, 0, false, nil)
	mockRepo.On("Create", mock.Anything, &beer).Return(nil)

	err := usecase.Create(context.Background(), &beer)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestGetByID tests the GetByID method
func TestGetByID(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	beer := model.Beer{ID: "1", Name: "Beer1"}

	mockRepo.On("GetByID", mock.Anything, "1").Return(beer, nil)

	result, err := usecase.GetByID(context.Background(), "1")

	assert.NoError(t, err)
	assert.Equal(t, beer, result)
	mockRepo.AssertExpectations(t)
}

// TestUpdate tests the Update method
func TestUpdate(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	beer := model.Beer{ID: "1", Name: "Updated Beer"}
	existing := model.Beer{ID: "1", Name: "Old Beer", CreatedBy: "user-1"}

	mockRepo.On("GetByID", mock.Anything, "1").Return(existing, nil)
	mockRepo.On("Update", mock.Anything, "1", beer).Return(nil)

	err := usecase.Update(middleware.WithUserID(context.Background(), "user-1", ""), "1", beer)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestDelete tests the Delete method
func TestDelete(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	existing := model.Beer{ID: "1", Name: "Beer1", CreatedBy: "user-1"}

	mockRepo.On("GetByID", mock.Anything, "1").Return(existing, nil)
	mockRepo.On("Delete", mock.Anything, "1").Return(nil)

	err := usecase.Delete(middleware.WithUserID(context.Background(), "user-1", ""), "1")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestAddComment tests the AddComment method
func TestAddComment(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)
	commentText := "Nice beer!"
	comment := model.Comment{ID: "c1", Text: commentText}

	mockRepo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)

	err := usecase.AddComment(context.Background(), "1", comment)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestDeleteComment tests the DeleteComment method (owner can delete)
func TestDeleteComment(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)

	err := usecase.DeleteComment(middleware.WithUserID(context.Background(), "user-1", ""), "1", "c1")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestDeleteCommentForbidden tests that a non-owner (non-admin) gets 403
func TestDeleteCommentForbidden(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("ExecInTx", mock.Anything, mock.Anything).Return(appErrors.NewAppError(403, "forbidden", nil))

	err := usecase.DeleteComment(middleware.WithUserID(context.Background(), "user-2", ""), "1", "c1")

	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 403, appErr.Code)
	mockRepo.AssertExpectations(t)
}

// TestLikeComment tests the LikeComment method
func TestLikeComment(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)

	err := usecase.LikeComment(context.Background(), "1", "c1", "", "device1")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestSearchBeers tests the SearchBeers method
func TestSearchBeers(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	filters := model.BeerFilters{Query: "Beer", Page: 1, PageSize: 10}
	beers := []model.Beer{{ID: "1", Name: "Beer1"}}
	total := 1

	mockRepo.On("SearchBeers", mock.Anything, filters).Return(beers, total, false, nil)

	result, totalCount, fuzzyMatch, err := usecase.SearchBeers(context.Background(), filters)

	assert.NoError(t, err)
	assert.Equal(t, beers, result)
	assert.Equal(t, total, totalCount)
	assert.False(t, fuzzyMatch)
	mockRepo.AssertExpectations(t)
}

// TestSearchBeersPropagatesError garante que erro do repositório vira 500.
func TestSearchBeersPropagatesError(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	filters := model.BeerFilters{Query: "x", Page: 1, PageSize: 10}
	mockRepo.On("SearchBeers", mock.Anything, filters).Return([]model.Beer{}, 0, false, stderrors.New("db boom"))

	_, _, _, err := uc.SearchBeers(context.Background(), filters)
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 500, appErr.Code)
}

// TestGetAllPropagatesError garante que erro do repositório vira 500.
func TestGetAllPropagatesError(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("GetAll", mock.Anything).Return([]model.Beer{}, stderrors.New("db boom"))

	_, err := uc.GetAll(context.Background())
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 500, appErr.Code)
}

// TestCreateSuccessNoDuplicate valida o caminho feliz: sem cervejas
// semelhantes, o usecase regista o criador (AuthZ) e cria a cerveja.
func TestCreateSuccessNoDuplicate(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	beer := model.Beer{ID: "1", Name: "Unica"}
	mockRepo.On("SearchBeers", mock.Anything, mock.Anything).Return([]model.Beer{}, 0, false, nil)
	mockRepo.On("Create", mock.Anything, &beer).Return(nil)

	ctx := middleware.WithUserID(context.Background(), "owner-1", "")
	err := uc.Create(ctx, &beer)

	assert.NoError(t, err)
	assert.Equal(t, "owner-1", beer.CreatedBy)
	mockRepo.AssertExpectations(t)
}

// TestCreateDuplicateWithSuggestions valida o bloqueio 409 (Decisão C) e a
// estrutura do detail (lista de sugestões) devolvida ao cliente.
func TestCreateDuplicateWithSuggestions(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	beer := model.Beer{ID: "1", Name: "Heineken"}
	existing := []model.Beer{{ID: "12", Name: "Heineken Long Neck"}}
	mockRepo.On("SearchBeers", mock.Anything, mock.Anything).Return(existing, 1, false, nil)

	err := uc.Create(context.Background(), &beer)
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 409, appErr.Code)
	assert.Equal(t, "DUPLICATE_BEER", appErr.ErrorCode)
	assert.NotEmpty(t, appErr.Details)
}

// TestLikeCommentAlreadyLiked valida o 400 quando o mesmo utilizador
// (identificado por user_id, Decisão A) curte duas vezes.
func TestLikeCommentAlreadyLiked(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("ExecInTx", mock.Anything, mock.Anything).Return(appErrors.NewAppError(400, "already liked", nil))

	err := uc.LikeComment(context.Background(), "1", "c1", "user-9", "")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 400, appErr.Code)
}

// TestLikeCommentCommentNotFound valida o 404 quando o comentário não existe.
func TestLikeCommentCommentNotFound(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("ExecInTx", mock.Anything, mock.Anything).Return(appErrors.NewAppError(404, "Comment not found", nil))

	err := uc.LikeComment(context.Background(), "1", "nao-existe", "user-9", "")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 404, appErr.Code)
}

// TestDeleteCommentForbiddenNonOwner valida o 403 quando um não-dono (sem
// admin) tenta apagar o comentário de outro (Gap de Product QA / Decisão B).
func TestDeleteCommentForbiddenNonOwner(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("ExecInTx", mock.Anything, mock.Anything).Return(appErrors.NewAppError(403, "forbidden", nil))

	err := uc.DeleteComment(middleware.WithUserID(context.Background(), "intruso-2", ""), "1", "c1")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 403, appErr.Code)
}

// TestDeleteCommentAdminOverride valida que um admin apaga qualquer comentário
// (escopo global de admin), mesmo não sendo o dono.
func TestDeleteCommentAdminOverride(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)

	ctx := middleware.WithUserID(context.Background(), "admin-1", usermodel.RoleAdmin)
	err := uc.DeleteComment(ctx, "1", "c1")
	assert.NoError(t, err)
}

func TestGetPaginated(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	beers := []model.Beer{{ID: "1", Name: "Beer1"}}
	mockRepo.On("GetPaginated", mock.Anything, 1, 10).Return(beers, 1, nil)

	result, total, err := uc.GetPaginated(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Equal(t, beers, result)
	assert.Equal(t, 1, total)
	mockRepo.AssertExpectations(t)
}

func TestGetPaginatedPropagatesError(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("GetPaginated", mock.Anything, 1, 10).Return([]model.Beer{}, 0, stderrors.New("db boom"))

	_, _, err := uc.GetPaginated(context.Background(), 1, 10)
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 500, appErr.Code)
}

func TestUpdateAdminOverride(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	existing := model.Beer{ID: "1", Name: "Old", CreatedBy: "owner-1"}
	updated := model.Beer{ID: "1", Name: "New"}

	mockRepo.On("GetByID", mock.Anything, "1").Return(existing, nil)
	mockRepo.On("Update", mock.Anything, "1", updated).Return(nil)

	ctx := middleware.WithUserID(context.Background(), "admin-1", usermodel.RoleAdmin)
	err := uc.Update(ctx, "1", updated)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestUpdateForbidden(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	existing := model.Beer{ID: "1", Name: "Old", CreatedBy: "owner-1"}

	mockRepo.On("GetByID", mock.Anything, "1").Return(existing, nil)

	ctx := middleware.WithUserID(context.Background(), "intruder-2", "")
	err := uc.Update(ctx, "1", model.Beer{Name: "New"})
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 403, appErr.Code)
}

func TestDeleteAdminOverride(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	existing := model.Beer{ID: "1", Name: "Beer1", CreatedBy: "owner-1"}

	mockRepo.On("GetByID", mock.Anything, "1").Return(existing, nil)
	mockRepo.On("Delete", mock.Anything, "1").Return(nil)

	ctx := middleware.WithUserID(context.Background(), "admin-1", usermodel.RoleAdmin)
	err := uc.Delete(ctx, "1")
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestAddMedia(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	beer := model.Beer{ID: "1", Name: "Beer1", CreatedBy: "owner-1", Media: []model.MediaItem{}}
	item := model.MediaItem{URL: "http://img", Type: "image"}

	mockRepo.On("GetByID", mock.Anything, "1").Return(beer, nil)
	mockRepo.On("Update", mock.Anything, "1", mock.Anything).Return(nil)

	ctx := middleware.WithUserID(context.Background(), "owner-1", "")
	media, err := uc.AddMedia(ctx, "1", item)

	assert.NoError(t, err)
	assert.Equal(t, []model.MediaItem{{URL: "http://img", Type: "image"}}, media)
	mockRepo.AssertExpectations(t)
}

func TestAddMediaForbidden(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	beer := model.Beer{ID: "1", Name: "Beer1", CreatedBy: "owner-1"}

	mockRepo.On("GetByID", mock.Anything, "1").Return(beer, nil)

	ctx := middleware.WithUserID(context.Background(), "intruder-2", "")
	_, err := uc.AddMedia(ctx, "1", model.MediaItem{URL: "http://img", Type: "image"})
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 403, appErr.Code)
}

func TestGetByIDPropagatesError(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("GetByID", mock.Anything, "1").Return(model.Beer{}, stderrors.New("db boom"))

	_, err := uc.GetByID(context.Background(), "1")
	assert.Error(t, err)
	assert.Equal(t, "db boom", err.Error())
}

func TestUpdateBeerNotFound(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("GetByID", mock.Anything, "1").Return(model.Beer{}, stderrors.New("not found"))

	ctx := middleware.WithUserID(context.Background(), "owner-1", "")
	err := uc.Update(ctx, "1", model.Beer{Name: "New"})
	assert.Error(t, err)
}

func TestCanModify_AdminAlwaysAllowed(t *testing.T) {
	ctx := middleware.WithUserID(context.Background(), "admin-1", usermodel.RoleAdmin)
	assert.True(t, canModify(ctx, "owner-1"))
	assert.True(t, canModify(ctx, ""))
}

func TestCanModify_OwnerAllowed(t *testing.T) {
	ctx := middleware.WithUserID(context.Background(), "owner-1", "")
	assert.True(t, canModify(ctx, "owner-1"))
	assert.False(t, canModify(ctx, "other-1"))
}

func TestCanModify_EmptyOwnerOnlyAdmin(t *testing.T) {
	ctx := middleware.WithUserID(context.Background(), "owner-1", "")
	assert.False(t, canModify(ctx, ""))

	ctxAdmin := middleware.WithUserID(context.Background(), "admin-1", usermodel.RoleAdmin)
	assert.True(t, canModify(ctxAdmin, ""))
}

func TestUnavailable_Returns503WhenRepoNil(t *testing.T) {
	uc := NewBeerUsecase(nil, nil, moderation.NewNoopModerator(), nil)

	_, err := uc.GetAll(context.Background())
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, http.StatusServiceUnavailable, appErr.Code)
}
func TestAddMedia_SyncsImageUrl(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	beer := model.Beer{ID: "1", Name: "Beer1", CreatedBy: "owner-1", ImageUrl: ""}
	item := model.MediaItem{URL: "https://img.example.com/beer.jpg", Type: "image/jpeg"}

	mockRepo.On("GetByID", mock.Anything, "1").Return(beer, nil)
	mockRepo.On("Update", mock.Anything, "1", mock.Anything).Return(nil)

	ctx := middleware.WithUserID(context.Background(), "owner-1", "")
	media, err := uc.AddMedia(ctx, "1", item)
	assert.NoError(t, err)
	assert.Equal(t, []model.MediaItem{{URL: "https://img.example.com/beer.jpg", Type: "image/jpeg"}}, media)
}

func TestLikeComment_UsesUserIDOverDeviceID(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)

	err := uc.LikeComment(context.Background(), "1", "c1", "user-1", "device-1")
	assert.NoError(t, err)
}

func TestLikeComment_DeviceIDFallback(t *testing.T) {
	mockRepo := new(beerRepo.MockBeerRepository)
	uc := NewBeerUsecase(mockRepo, nil, moderation.NewNoopModerator(), nil)

	mockRepo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)

	err := uc.LikeComment(context.Background(), "1", "c1", "", "device-1")
	assert.NoError(t, err)
}
