package model

type Comment struct {
	ID       string   `json:"id"`
	BeerID   string   `json:"beerId"`
	UserID   string   `json:"userId"`
	Text     string   `json:"text" validate:"required"`
	Likes    int      `json:"likes"`
	LikedBy  []string `json:"likedBy"`
	Positive bool     `json:"positive" validate:"required"`
	Created  string   `json:"created"`
}
