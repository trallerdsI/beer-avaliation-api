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
	ID        string   `json:"id"`
	Text      string   `json:"text" validate:"required,max=2000"`
	Likes     int      `json:"likes"`
	LikedBy   []string `json:"likedBy"` // Store device/user IDs that liked this comment
	Positive  bool     `json:"positive" validate:"required"`
	CreatedBy string   `json:"createdBy"` // user_id do autor do comentário (AuthZ)
	CreatedAt string   `json:"createdAt"` // timestamp ISO8601 para ordenação cronológica do feed
}

// Beer represents a beer object.
// Todas as colunas da tabela `beers` são NOT NULL (incluindo description e
// image_url, que passaram por ALTER TABLE). Por isso todos os campos de domínio
// são obrigatórios (required) e sem defaults: o cliente deve enviar o payload
// completo ou recebe 400.
type Beer struct {
	ID          string      `json:"id"`
	Name        string      `json:"name" validate:"required,min=3,max=100"`
	Style       string      `json:"style" validate:"omitempty"`
	Description string      `json:"description" validate:"max=500"`
	ImageUrl    string      `json:"imageUrl" validate:"omitempty,https_url"`
	Alcohol     *float64    `json:"alcohol" validate:"omitempty,min=0,max=100"`
	Taste       Flavor      `json:"taste" validate:"omitempty,flavor"`
	Aroma       Aroma       `json:"aroma" validate:"omitempty,aroma"`
	Color       Color       `json:"color" validate:"omitempty,color"`
	Body        Body        `json:"body" validate:"omitempty,body"`
	Carbonation Carbonation `json:"carbonation" validate:"omitempty,carbonation"`
	Finish      Finish      `json:"finish" validate:"omitempty,finish"`
	Comments    []Comment   `json:"comments"`
	CreatedBy   string      `json:"createdBy"` // user_id do criador (AuthZ: só criador ou admin editam)
	CreatedAt   string      `json:"createdAt"` // timestamp ISO8601 de criação da cerveja
}

// Allowlists de valores válidos para cada enum do domínio. O backend é a
// fonte da verdade: o app consome estas listas (via GET /api/v1/enums) e não
// aceita entrada livre do utilizador.
var (
	flavorValues      = map[Flavor]bool{}
	aromaValues       = map[Aroma]bool{}
	colorValues       = map[Color]bool{}
	bodyValues        = map[Body]bool{}
	carbonationValues = map[Carbonation]bool{}
	finishValues      = map[Finish]bool{}
)

func init() {
	for _, v := range []Flavor{FlavorSweet, FlavorBitter, FlavorFruity, FlavorSpicy, FlavorSavory, FlavorSour} {
		flavorValues[v] = true
	}
	for _, v := range []Aroma{AromaFloral, AromaFruity, AromaMalty, AromaSpicy, AromaEarthy, AromaCitrus, AromaToasty} {
		aromaValues[v] = true
	}
	for _, v := range []Color{ColorPale, ColorAmber, ColorBrown, ColorDark, ColorBlack} {
		colorValues[v] = true
	}
	for _, v := range []Body{BodyLight, BodyMedium, BodyFull} {
		bodyValues[v] = true
	}
	for _, v := range []Carbonation{CarbonationLow, CarbonationMedium, CarbonationHigh, CarbonationNatural, CarbonationForced} {
		carbonationValues[v] = true
	}
	for _, v := range []Finish{FinishDry, FinishSweet, FinishBitter, FinishClean, FinishLingering} {
		finishValues[v] = true
	}
}

// EnumValues devolve as listas de valores aceites para o endpoint /enums.
func EnumValues() map[string][]string {
	keys := func(m interface{ Keys() []string }) []string { return m.Keys() }
	_ = keys
	return map[string][]string{
		"flavor":      flavorValuesSlice(),
		"aroma":       aromaValuesSlice(),
		"color":       colorValuesSlice(),
		"body":        bodyValuesSlice(),
		"carbonation": carbonationValuesSlice(),
		"finish":      finishValuesSlice(),
	}
}

func flavorValuesSlice() []string {
	out := make([]string, 0, len(flavorValues))
	for k := range flavorValues {
		out = append(out, string(k))
	}
	return out
}
func aromaValuesSlice() []string {
	out := make([]string, 0, len(aromaValues))
	for k := range aromaValues {
		out = append(out, string(k))
	}
	return out
}
func colorValuesSlice() []string {
	out := make([]string, 0, len(colorValues))
	for k := range colorValues {
		out = append(out, string(k))
	}
	return out
}
func bodyValuesSlice() []string {
	out := make([]string, 0, len(bodyValues))
	for k := range bodyValues {
		out = append(out, string(k))
	}
	return out
}
func carbonationValuesSlice() []string {
	out := make([]string, 0, len(carbonationValues))
	for k := range carbonationValues {
		out = append(out, string(k))
	}
	return out
}
func finishValuesSlice() []string {
	out := make([]string, 0, len(finishValues))
	for k := range finishValues {
		out = append(out, string(k))
	}
	return out
}

// isOneOf confere se o valor pertence à allowlist tipada.
func isOneOf[T ~string](m map[T]bool, v T) bool {
	return m[v]
}

// Predicados de validação de enum expostos para o controller registrar no
// validator. Conferem se o valor pertence à allowlist.
func IsFlavor(v Flavor) bool           { return isOneOf(flavorValues, v) }
func IsAroma(v Aroma) bool             { return isOneOf(aromaValues, v) }
func IsColor(v Color) bool             { return isOneOf(colorValues, v) }
func IsBody(v Body) bool               { return isOneOf(bodyValues, v) }
func IsCarbonation(v Carbonation) bool { return isOneOf(carbonationValues, v) }
func IsFinish(v Finish) bool           { return isOneOf(finishValues, v) }
