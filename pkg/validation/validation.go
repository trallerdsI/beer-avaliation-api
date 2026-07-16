package validation

import "beer-review-app/internal/beer/model"

// Conjuntos de valores válidos pré-computados em init: lookup O(1) sem
// alocação por chamada (Pilar 2 — zero-allocation no hot path de validação).
var (
	validFlavors  = map[model.Flavor]bool{}
	validAromas   = map[model.Aroma]bool{}
	validColors   = map[model.Color]bool{}
	validBodies   = map[model.Body]bool{}
	validCarbs    = map[model.Carbonation]bool{}
	validFinishes = map[model.Finish]bool{}
)

func init() {
	for _, v := range []model.Flavor{
		model.FlavorSweet, model.FlavorBitter, model.FlavorFruity,
		model.FlavorSpicy, model.FlavorSavory, model.FlavorSour,
	} {
		validFlavors[v] = true
	}
	for _, v := range []model.Aroma{
		model.AromaFloral, model.AromaFruity, model.AromaMalty,
		model.AromaSpicy, model.AromaEarthy, model.AromaCitrus, model.AromaToasty,
	} {
		validAromas[v] = true
	}
	for _, v := range []model.Color{
		model.ColorPale, model.ColorAmber, model.ColorBrown,
		model.ColorDark, model.ColorBlack,
	} {
		validColors[v] = true
	}
	for _, v := range []model.Body{
		model.BodyLight, model.BodyMedium, model.BodyFull,
	} {
		validBodies[v] = true
	}
	for _, v := range []model.Carbonation{
		model.CarbonationLow, model.CarbonationMedium, model.CarbonationHigh,
		model.CarbonationNatural, model.CarbonationForced,
	} {
		validCarbs[v] = true
	}
	for _, v := range []model.Finish{
		model.FinishDry, model.FinishSweet, model.FinishBitter,
		model.FinishClean, model.FinishLingering,
	} {
		validFinishes[v] = true
	}
}

// IsValidFlavor checks if the given flavor is valid.
func IsValidFlavor(flavor model.Flavor) bool { return validFlavors[flavor] }

// IsValidAroma checks if the given aroma is valid.
func IsValidAroma(aroma model.Aroma) bool { return validAromas[aroma] }

// IsValidColor checks if the given color is valid.
func IsValidColor(color model.Color) bool { return validColors[color] }

// IsValidBody checks if the given body is valid.
func IsValidBody(body model.Body) bool { return validBodies[body] }

// IsValidCarbonation checks if the given carbonation level is valid.
func IsValidCarbonation(carbonation model.Carbonation) bool { return validCarbs[carbonation] }

// IsValidFinish checks if the given finish is valid.
func IsValidFinish(finish model.Finish) bool { return validFinishes[finish] }
