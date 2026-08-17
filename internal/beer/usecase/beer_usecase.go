package usecase

import (
	"context"
	"time"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/repository"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/realtime"
	"beer-review-app/pkg/uuid"
)

type BeerUsecase interface {
	GetAll(ctx context.Context) ([]model.Beer, error)
	Create(ctx context.Context, beer *model.Beer) error
	GetByID(ctx context.Context, id string) (model.Beer, error)
	GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error)
	Update(ctx context.Context, id string, beer model.Beer) error
	Delete(ctx context.Context, id string) error
	AddComment(ctx context.Context, id string, comment model.Comment) error
	DeleteComment(ctx context.Context, id string, commentID string) error
	LikeComment(ctx context.Context, beerID, commentID, userID, deviceID string) error
	AddMedia(ctx context.Context, id string, item model.MediaItem) ([]model.MediaItem, error)
	SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error)
}

type beerUsecase struct {
	repo repository.BeerRepository
	hub  *realtime.Hub // opcional: nil em testes/serverless desativa eventos SSE
}

// NewBeerUsecase creates a new instance of BeerUsecase.
// hub pode ser nil (ex: testes, serverless) — neste caso nenhum evento SSE é emitido.
func NewBeerUsecase(repo repository.BeerRepository, hub *realtime.Hub) BeerUsecase {
	return &beerUsecase{
		repo: repo,
		hub:  hub,
	}
}

// publish emite um evento SSE se o hub estiver configurado. Non-blocking:
// o Hub descarta sob back-pressure, protegendo a memória do servidor.
func (u *beerUsecase) publish(ev realtime.Event) {
	if u.hub != nil {
		u.hub.Publish(ev)
	}
}

func (u *beerUsecase) unavailable() error {
	if u.repo == nil {
		return errors.NewUnavailableError()
	}
	return nil
}

// GetAll retrieves beers from repository.
func (u *beerUsecase) GetAll(ctx context.Context) ([]model.Beer, error) {
	if err := u.unavailable(); err != nil {
		return nil, err
	}
	// Retrieve beers from the repository
	beers, err := u.repo.GetAll(ctx)
	if err != nil {
		return nil, errors.NewAppError(500, "Failed to retrieve beers", err)
	}

	return beers, nil
}

// Create adds a new beer.
func (u *beerUsecase) Create(ctx context.Context, beer *model.Beer) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	// UUIDv7 (RFC 9562): id time-ordered gerado na app, antes do repo.
	if beer.ID == "" {
		beer.ID = uuid.MustNewV7()
	}
	// AuthZ: regista o criador (qualquer user logado pode criar cerveja global).
	if uid, ok := middleware.UserIDFromContext(ctx); ok {
		beer.CreatedBy = uid
	}
	// Timestamp de criação para ordenação cronológica do feed social.
	beer.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	// Decisão C: evita poluição do catálogo. Se já existe cerveja com nome
	// semelhante, bloqueia com 409 e devolve sugestões para o utilizador
	// comentar na existente em vez de duplicar.
	if existing, _, err := u.repo.SearchBeers(ctx, model.BeerFilters{
		Query:    beer.Name,
		Page:     1,
		PageSize: 5,
	}); err == nil && len(existing) > 0 {
		details := make([]errors.ProblemDetail, 0, len(existing))
		for _, b := range existing {
			details = append(details, errors.ProblemDetail{
				Code:   "duplicate_beer",
				Detail: b.Name,
			})
		}
		return errors.NewAppErrorWithDetails(409, "Uma cerveja com nome semelhante já existe", "DUPLICATE_BEER", details)
	}

	if err := u.repo.Create(ctx, beer); err != nil {
		return errors.NewAppError(500, "Failed to create beer", err)
	}

	// Evento SSE: notifica clientes móveis em tempo real (sem polling).
	u.publish(realtime.Event{
		Type: "beer.created",
		ID:   beer.ID,
		Data: map[string]any{"name": beer.Name, "style": beer.Style},
	})

	return nil
}

// GetByID retrieves a beer by its ID.
func (u *beerUsecase) GetByID(ctx context.Context, id string) (model.Beer, error) {
	if err := u.unavailable(); err != nil {
		return model.Beer{}, err
	}
	return u.repo.GetByID(ctx, id)
}

// GetPaginated retrieves paginated beers.
func (u *beerUsecase) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	if err := u.unavailable(); err != nil {
		return nil, 0, err
	}
	beers, total, err := u.repo.GetPaginated(ctx, page, pageSize)
	if err != nil {
		return nil, 0, errors.NewAppError(500, "Failed to retrieve paginated beers", err)
	}
	return beers, total, nil
}

// Update updates an existing beer. AuthZ: só o criador ou um admin podem
// editar. created_by NULL (cervejas seedadas por scrap) só pode ser editado por admin.
func (u *beerUsecase) Update(ctx context.Context, id string, beer model.Beer) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	existing, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !canModify(ctx, existing.CreatedBy) {
		return errors.NewAppError(403, "you are not allowed to update this beer", nil)
	}

	if err := u.repo.Update(ctx, id, beer); err != nil {
		return errors.NewAppError(500, "Failed to update beer", err)
	}
	return nil
}

// Delete removes a beer. AuthZ: só o criador ou um admin podem apagar.
func (u *beerUsecase) Delete(ctx context.Context, id string) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	existing, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !canModify(ctx, existing.CreatedBy) {
		return errors.NewAppError(403, "you are not allowed to delete this beer", nil)
	}

	if err := u.repo.Delete(ctx, id); err != nil {
		return errors.NewAppError(500, "Failed to delete beer", err)
	}
	return nil
}

// canModify devolve true se o chamador (do contexto) pode editar/apagar o
// recurso cujo dono é ownerID. Regra: admin sempre pode; caso contrário o
// user deve ser o dono. created_by vazio (seed por scrap) => só admin.
func canModify(ctx context.Context, ownerID string) bool {
	if middleware.IsAdmin(ctx) {
		return true
	}
	if ownerID == "" {
		return false
	}
	uid, ok := middleware.UserIDFromContext(ctx)
	return ok && uid == ownerID
}

func (u *beerUsecase) AddComment(ctx context.Context, id string, comment model.Comment) error {
	if err := u.unavailable(); err != nil {
		return err
	}

	if err := u.repo.ExecInTx(ctx, func(ctx context.Context, txRepo repository.BeerRepository) error {
		beer, err := txRepo.GetByID(ctx, id)
		if err != nil {
			return errors.NewAppError(404, "Beer not found", err)
		}

		beer.Comments = append(beer.Comments, comment)
		if err := txRepo.Update(ctx, id, beer); err != nil {
			return errors.NewAppError(500, "Failed to update beer with new comment", err)
		}
		return nil
	}); err != nil {
		return err
	}

	u.publish(realtime.Event{
		Type: "comment.added",
		ID:   id,
		Data: map[string]any{"commentId": comment.ID, "text": comment.Text},
	})

	return nil
}

func (u *beerUsecase) DeleteComment(ctx context.Context, id string, commentID string) error {
	if err := u.unavailable(); err != nil {
		return err
	}

	var owner string
	if err := u.repo.ExecInTx(ctx, func(ctx context.Context, txRepo repository.BeerRepository) error {
		beer, err := txRepo.GetByID(ctx, id)
		if err != nil {
			return errors.NewAppError(404, "Beer not found", err)
		}

		var found bool
		for i, c := range beer.Comments {
			if c.ID == commentID {
				owner = c.CreatedBy
				beer.Comments = append(beer.Comments[:i], beer.Comments[i+1:]...)
				found = true
				break
			}
		}

		if !found {
			return errors.NewAppError(404, "comment not found", nil)
		}

		if !canModify(ctx, owner) {
			return errors.NewAppError(403, "you are not allowed to delete this comment", nil)
		}

		if err := txRepo.Update(ctx, id, beer); err != nil {
			return errors.NewAppError(500, "Failed to update beer after deleting comment", err)
		}
		return nil
	}); err != nil {
		return err
	}

	u.publish(realtime.Event{
		Type: "comment.deleted",
		ID:   id,
		Data: map[string]any{"commentId": commentID},
	})

	return nil
}

// LikeComment regista um like num comentário. Num app social com login, o like
// é identificado pelo user_id autenticado (previne múltiplos likes por
// dispositivo e permite toggling consistente). Para utilizadores anónimos (sem
// login), usa o deviceID como fallback, mantendo o comportamento anterior.
func (u *beerUsecase) LikeComment(ctx context.Context, beerID, commentID, userID, deviceID string) error {
	if err := u.unavailable(); err != nil {
		return err
	}

	var likes int
	if err := u.repo.ExecInTx(ctx, func(ctx context.Context, txRepo repository.BeerRepository) error {
		beer, err := txRepo.GetByID(ctx, beerID)
		if err != nil {
			return errors.NewAppError(404, "Beer not found", err)
		}

		liker := deviceID
		if userID != "" {
			liker = "u:" + userID
		}

		for i, comment := range beer.Comments {
			if comment.ID == commentID {
				for _, id := range comment.LikedBy {
					if id == liker {
						return errors.NewAppError(400, "already liked", nil)
					}
				}

				beer.Comments[i].Likes++
				beer.Comments[i].LikedBy = append(beer.Comments[i].LikedBy, liker)
				likes = beer.Comments[i].Likes

				if err := txRepo.Update(ctx, beerID, beer); err != nil {
					return errors.NewAppError(500, "Failed to update comment likes", err)
				}
				return nil
			}
		}

		return errors.NewAppError(404, "Comment not found", nil)
	}); err != nil {
		return err
	}

	u.publish(realtime.Event{
		Type: "comment.liked",
		ID:   beerID,
		Data: map[string]any{"commentId": commentID, "likes": likes},
	})

	return nil
}

// AddMedia anexa um item de mídia (imagem já carregada no storage) à cerveja.
// AuthZ: só o criador ou um admin podem anexar (mesma regra de edição).
// Devolve a lista atualizada de mídias para o controller responder.
func (u *beerUsecase) AddMedia(ctx context.Context, id string, item model.MediaItem) ([]model.MediaItem, error) {
	if err := u.unavailable(); err != nil {
		return nil, err
	}
	beer, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canModify(ctx, beer.CreatedBy) {
		return nil, errors.NewAppError(403, "you are not allowed to modify this beer", nil)
	}

	beer.Media = append(beer.Media, item)
	// Mantém image_url em sincronia com a primeira mídia (compat com clientes antigos).
	if beer.ImageUrl == "" {
		beer.ImageUrl = item.URL
	}

	if err := u.repo.Update(ctx, id, beer); err != nil {
		return nil, errors.NewAppError(500, "Failed to attach media", err)
	}

	u.publish(realtime.Event{
		Type: "beer.media.added",
		ID:   id,
		Data: map[string]any{"url": item.URL, "type": item.Type},
	})

	return beer.Media, nil
}

// SearchBeers searches for beers using the provided filters
func (u *beerUsecase) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error) {
	if err := u.unavailable(); err != nil {
		return nil, 0, err
	}
	// get from repository
	beers, total, err := u.repo.SearchBeers(ctx, filters)
	if err != nil {
		return nil, 0, errors.NewAppError(500, "Failed to search beers", err)
	}

	return beers, total, nil
}
