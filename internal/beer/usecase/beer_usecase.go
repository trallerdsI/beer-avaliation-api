package usecase

import (
	"context"
	"time"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/repository"
	"beer-review-app/pkg/errors"

	"github.com/sony/gobreaker"
)

type BeerUsecase interface {
	GetAll(ctx context.Context) ([]model.Beer, error)
	Create(ctx context.Context, beer model.Beer) error
	GetByID(ctx context.Context, id string) (model.Beer, error)
	GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error)
	Update(ctx context.Context, id string, beer model.Beer) error
	Delete(ctx context.Context, id string) error
	AddComment(ctx context.Context, id string, comment model.Comment) error
	DeleteComment(ctx context.Context, id string, commentID string) error
	LikeComment(ctx context.Context, beerID, commentID, deviceID string) error
	SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error)
}

type beerUsecase struct {
	repo repository.BeerRepository
	cb   *gobreaker.CircuitBreaker
}

// NewBeerUsecase creates a new instance of BeerUsecase.
func NewBeerUsecase(repo repository.BeerRepository) BeerUsecase {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "beer-service",
		MaxRequests: 5,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
	})

	return &beerUsecase{
		repo: repo,
		cb:   cb,
	}
}

// GetAll retrieves beers from repository.
func (u *beerUsecase) GetAll(ctx context.Context) ([]model.Beer, error) {
	// Retrieve beers from the repository
	beers, err := u.repo.GetAll(ctx)
	if err != nil {
		return nil, errors.NewAppError(500, "Failed to retrieve beers", err)
	}

	return beers, nil
}

// Create adds a new beer.
func (u *beerUsecase) Create(ctx context.Context, beer model.Beer) error {
	if err := u.repo.Create(ctx, beer); err != nil {
		return errors.NewAppError(500, "Failed to create beer", err)
	}

	return nil
}

// GetByID retrieves a beer by its ID using circuit breaker pattern.
func (u *beerUsecase) GetByID(ctx context.Context, id string) (model.Beer, error) {
	result, err := u.cb.Execute(func() (interface{}, error) {
		return u.repo.GetByID(ctx, id)
	})
	if err != nil {
		return model.Beer{}, errors.NewAppError(500, "Failed to retrieve beer by ID", err)
	}

	return result.(model.Beer), nil
}

// GetPaginated retrieves paginated beers.
func (u *beerUsecase) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	beers, total, err := u.repo.GetPaginated(ctx, page, pageSize)
	if err != nil {
		return nil, 0, errors.NewAppError(500, "Failed to retrieve paginated beers", err)
	}
	return beers, total, nil
}

// Update updates an existing beer
func (u *beerUsecase) Update(ctx context.Context, id string, beer model.Beer) error {
	if err := u.repo.Update(ctx, id, beer); err != nil {
		return errors.NewAppError(500, "Failed to update beer", err)
	}
	return nil
}

// Delete removes a beer
func (u *beerUsecase) Delete(ctx context.Context, id string) error {
	if err := u.repo.Delete(ctx, id); err != nil {
		return errors.NewAppError(500, "Failed to delete beer", err)
	}
	return nil
}

func (u *beerUsecase) AddComment(ctx context.Context, id string, comment model.Comment) error {
	beer, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return errors.NewAppError(404, "Beer not found", err)
	}

	beer.Comments = append(beer.Comments, comment)
	if err := u.repo.Update(ctx, id, beer); err != nil {
		return errors.NewAppError(500, "Failed to update beer with new comment", err)
	}
	return nil
}

func (u *beerUsecase) DeleteComment(ctx context.Context, id string, commentID string) error {
	beer, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return errors.NewAppError(404, "Beer not found", err)
	}

	for i, c := range beer.Comments {
		if c.ID == commentID {
			beer.Comments = append(beer.Comments[:i], beer.Comments[i+1:]...)
			break
		}
	}

	if err := u.repo.Update(ctx, id, beer); err != nil {
		return errors.NewAppError(500, "Failed to update beer after deleting comment", err)
	}
	return nil
}

func (u *beerUsecase) LikeComment(ctx context.Context, beerID, commentID, deviceID string) error {
	beer, err := u.repo.GetByID(ctx, beerID)
	if err != nil {
		return errors.NewAppError(404, "Beer not found", err)
	}

	for i, comment := range beer.Comments {
		if comment.ID == commentID {
			// Check if already liked by this device
			for _, id := range comment.LikedBy {
				if id == deviceID {
					return errors.NewAppError(400, "already liked", nil)
				}
			}

			// Add like
			beer.Comments[i].Likes++
			beer.Comments[i].LikedBy = append(beer.Comments[i].LikedBy, deviceID)

			if err := u.repo.Update(ctx, beerID, beer); err != nil {
				return errors.NewAppError(500, "Failed to update comment likes", err)
			}
			return nil
		}
	}

	return errors.NewAppError(404, "Comment not found", nil)
}

// SearchBeers searches for beers using the provided filters
func (u *beerUsecase) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error) {
	// get from repository
	beers, total, err := u.repo.SearchBeers(ctx, filters)
	if err != nil {
		return nil, 0, errors.NewAppError(500, "Failed to search beers", err)
	}

	return beers, total, nil
}
