package validation

import "beer-review-app/internal/beer/model"

// IsValidFlavor checks if the given flavor is valid.
func IsValidFlavor(flavor model.Flavor) bool {
    switch flavor {
    case model.FlavorSweet, model.FlavorBitter, model.FlavorFruity, model.FlavorSpicy, model.FlavorSavory, model.FlavorSour:
        return true
    }
    return false
}

// IsValidAroma checks if the given aroma is valid.
func IsValidAroma(aroma model.Aroma) bool {
    switch aroma {
    case model.AromaFloral, model.AromaFruity, model.AromaMalty, model.AromaSpicy, model.AromaEarthy, model.AromaCitrus, model.AromaToasty:
        return true
    }
    return false
}

// IsValidColor checks if the given color is valid.
func IsValidColor(color model.Color) bool {
    switch color {
    case model.ColorPale, model.ColorAmber, model.ColorBrown, model.ColorDark, model.ColorBlack:
        return true
    }
    return false
}

// IsValidBody checks if the given body is valid.
func IsValidBody(body model.Body) bool {
    switch body {
    case model.BodyLight, model.BodyMedium, model.BodyFull:
        return true
    }
    return false
}

// IsValidCarbonation checks if the given carbonation level is valid.
func IsValidCarbonation(carbonation model.Carbonation) bool {
    switch carbonation {
    case model.CarbonationLow, model.CarbonationMedium, model.CarbonationHigh, model.CarbonationNatural, model.CarbonationForced:
        return true
    }
    return false
}

// IsValidFinish checks if the given finish is valid.
func IsValidFinish(finish model.Finish) bool {
    switch finish {
    case model.FinishDry, model.FinishSweet, model.FinishBitter, model.FinishClean, model.FinishLingering:
        return true
    }
    return false
}
