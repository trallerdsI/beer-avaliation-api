package model

// Enums for different characteristics of beer
type (
	Flavor      string
	Aroma       string
	Color       string
	Body        string
	Carbonation string
	Finish      string
)

// Flavor constants
const (
	FlavorSweet  Flavor = "Doce"
	FlavorBitter Flavor = "Amargo"
	FlavorFruity Flavor = "Frutado"
	FlavorSpicy  Flavor = "Especiarias"
	FlavorSavory Flavor = "Salgado"
	FlavorSour   Flavor = "Azedo"
)

// Aroma constants
const (
	AromaFloral Aroma = "Floral"
	AromaFruity Aroma = "Frutado"
	AromaMalty  Aroma = "Malte"
	AromaSpicy  Aroma = "Especiarias"
	AromaEarthy Aroma = "Terroso"
	AromaCitrus Aroma = "Cítrico"
	AromaToasty Aroma = "Torrado"
)

// Color constants
const (
	ColorPale  Color = "Clara"
	ColorAmber Color = "Âmbar"
	ColorBrown Color = "Marrom"
	ColorDark  Color = "Escura"
	ColorBlack Color = "Preta"
)

// Body constants
const (
	BodyLight  Body = "Leve"
	BodyMedium Body = "Médio"
	BodyFull   Body = "Encorpado"
)

// Carbonation constants
const (
	CarbonationLow     Carbonation = "Baixa"
	CarbonationMedium  Carbonation = "Média"
	CarbonationHigh    Carbonation = "Alta"
	CarbonationNatural Carbonation = "Natural"
	CarbonationForced  Carbonation = "Forçada"
)

// Finish constants
const (
	FinishDry       Finish = "Seco"
	FinishSweet     Finish = "Doce"
	FinishBitter    Finish = "Amargo"
	FinishClean     Finish = "Limpo"
	FinishLingering Finish = "Persistente"
)

// Comment represents a comment with an ID, text, and metadata
type Comment struct {
	ID       string   `json:"id"`
	Text     string   `json:"text" validate:"required"`
	Likes    int      `json:"likes"`
	LikedBy  []string `json:"likedBy"` // Store device/user IDs that liked this comment
	Positive bool     `json:"positive" validate:"required"`
}

// Beer represents a beer object.
type Beer struct {
	ID          string      `json:"id"`
	Name        string      `json:"name" validate:"required,min=3,max=100"`
	Style       string      `json:"style" validate:"required"`
	Description string      `json:"description" validate:"max=500"`
	ImageUrl    string      `json:"imageUrl" validate:"omitempty,url"`
	Alcohol     *float64    `json:"alcohol,omitempty" validate:"omitempty,min=0,max=100"`
	Taste       Flavor      `json:"taste" validate:"required"`       // Ensure taste is included
	Aroma       Aroma       `json:"aroma" validate:"required"`       // Ensure aroma is included
	Color       Color       `json:"color" validate:"required"`       // Ensure color is included
	Body        Body        `json:"body" validate:"required"`        // Ensure body is included
	Carbonation Carbonation `json:"carbonation" validate:"required"` // Ensure carbonation is included
	Finish      Finish      `json:"finish" validate:"required"`      // Ensure finish is included
	Comments    []Comment   `json:"comments"`                         // Array of comments with ID and text
}
