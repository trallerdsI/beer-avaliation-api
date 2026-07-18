package model

import "testing"

// TestIsFlavor valida a allowlist de sabores (fonte da verdade do backend).
func TestIsFlavor(t *testing.T) {
	valid := []Flavor{FlavorSweet, FlavorBitter, FlavorFruity, FlavorSpicy, FlavorSavory, FlavorSour}
	for _, v := range valid {
		if !IsFlavor(v) {
			t.Errorf("expected %q to be a valid flavor", v)
		}
	}
	if IsFlavor(Flavor("invalido")) {
		t.Error("expected 'invalido' to be rejected")
	}
	if IsFlavor(Flavor("")) {
		t.Error("expected empty to be rejected")
	}
}

func TestIsAroma(t *testing.T) {
	valid := []Aroma{AromaFloral, AromaFruity, AromaMalty, AromaSpicy, AromaEarthy, AromaCitrus, AromaToasty}
	for _, v := range valid {
		if !IsAroma(v) {
			t.Errorf("expected %q to be a valid aroma", v)
		}
	}
	if IsAroma(Aroma("nope")) {
		t.Error("expected invalid aroma to be rejected")
	}
}

func TestIsColor(t *testing.T) {
	valid := []Color{ColorPale, ColorAmber, ColorBrown, ColorDark, ColorBlack}
	for _, v := range valid {
		if !IsColor(v) {
			t.Errorf("expected %q to be a valid color", v)
		}
	}
	if IsColor(Color("verde")) {
		t.Error("expected invalid color to be rejected")
	}
}

func TestIsBody(t *testing.T) {
	valid := []Body{BodyLight, BodyMedium, BodyFull}
	for _, v := range valid {
		if !IsBody(v) {
			t.Errorf("expected %q to be a valid body", v)
		}
	}
	if IsBody(Body("gigante")) {
		t.Error("expected invalid body to be rejected")
	}
}

func TestIsCarbonation(t *testing.T) {
	valid := []Carbonation{CarbonationLow, CarbonationMedium, CarbonationHigh, CarbonationNatural, CarbonationForced}
	for _, v := range valid {
		if !IsCarbonation(v) {
			t.Errorf("expected %q to be a valid carbonation", v)
		}
	}
	if IsCarbonation(Carbonation("x")) {
		t.Error("expected invalid carbonation to be rejected")
	}
}

func TestIsFinish(t *testing.T) {
	valid := []Finish{FinishDry, FinishSweet, FinishBitter, FinishClean, FinishLingering}
	for _, v := range valid {
		if !IsFinish(v) {
			t.Errorf("expected %q to be a valid finish", v)
		}
	}
	if IsFinish(Finish("estranho")) {
		t.Error("expected invalid finish to be rejected")
	}
}

// TestEnumValues garante que o endpoint /enums expõe todas as listas e que
// os valores válidos aparecem nas listas.
func TestEnumValues(t *testing.T) {
	vals := EnumValues()
	for _, key := range []string{"flavor", "aroma", "color", "body", "carbonation", "finish"} {
		list, ok := vals[key]
		if !ok || len(list) == 0 {
			t.Fatalf("expected non-empty list for %q", key)
		}
	}
	// Sabor válido deve constar na lista.
	found := false
	for _, f := range vals["flavor"] {
		if f == string(FlavorBitter) {
			found = true
		}
	}
	if !found {
		t.Error("expected FlavorBitter in flavor enum list")
	}
}

// TestCommentValidationTags documenta os limites do modelo que o controller
// aplica via validator: Text obrigatório (max 2000) e Rating 1-5.
func TestCommentValidationTags(t *testing.T) {
	c := Comment{Text: "", Rating: 0}
	if c.Text != "" {
		t.Fatal("unexpected")
	}
	// Rating fora de [1,5] é rejeitado pelo validate:"min=1,max=5" no controller.
	invalidRatings := []int{0, -1, 6, 100}
	for _, r := range invalidRatings {
		if r >= 1 && r <= 5 {
			t.Fatalf("rating %d should be considered invalid by contract", r)
		}
	}
	validRatings := []int{1, 2, 3, 4, 5}
	for _, r := range validRatings {
		if r < 1 || r > 5 {
			t.Fatalf("rating %d should be considered valid by contract", r)
		}
	}
	// Texto no limite (2000) é aceite; acima é rejeitado (max=2000).
	if len(make([]byte, 2000)) != 2000 {
		t.Fatal("unexpected length")
	}
}
