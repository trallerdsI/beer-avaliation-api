package validation

import (
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestFieldLabel(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		expected string
	}{
		{"known Name", "Name", "nome"},
		{"known Style", "Style", "estilo"},
		{"known Description", "Description", "descrição"},
		{"known ImageUrl", "ImageUrl", "imagem"},
		{"known Alcohol", "Alcohol", "teor alcoólico"},
		{"known Taste", "Taste", "sabor"},
		{"known Aroma", "Aroma", "aroma"},
		{"known Color", "Color", "cor"},
		{"known Body", "Body", "corpo"},
		{"known Carbonation", "Carbonation", "carbonatação"},
		{"known Finish", "Finish", "finalização"},
		{"known Text", "Text", "texto"},
		{"known Rating", "Rating", "avaliação"},
		{"known Comments", "Comments", "comentários"},
		{"known Username", "Username", "nome de utilizador"},
		{"known Email", "Email", "email"},
		{"known Password", "Password", "palavra-passe"},
		{"known Role", "Role", "perfil"},
		{"unknown returns itself", "UnknownField", "UnknownField"},
		{"empty returns empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FieldLabel(tt.field); got != tt.expected {
				t.Errorf("FieldLabel(%q) = %q, want %q", tt.field, got, tt.expected)
			}
		})
	}
}

func TestRuleMessage(t *testing.T) {
	type TestStruct struct {
		Name  string `validate:"required"`
		Age   int    `validate:"required,min=18,max=100"`
		Email string `validate:"required,email"`
		Other string `validate:"unknown_tag"`
	}

	validate := validator.New()
	_ = validate

	err := validate.RegisterValidation("unknown_tag", func(fl validator.FieldLevel) bool {
		return false
	})
	if err != nil {
		t.Fatalf("failed to register unknown_tag: %v", err)
	}

	tests := []struct {
		name     string
		input    TestStruct
		field    string
		tag      string
		expected string
	}{
		{"required string", TestStruct{Name: ""}, "Name", "required", "é obrigatório"},
		{"min numeric", TestStruct{Age: 10}, "Age", "min", "deve ser no mínimo 18"},
		{"max numeric", TestStruct{Age: 150}, "Age", "max", "deve ser no máximo 100"},
		{"email", TestStruct{Email: "invalid"}, "Email", "email", "deve ser um email válido"},
		{"default unknown tag", TestStruct{Other: "x"}, "Other", "unknown_tag", "valor inválido"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if err == nil {
				t.Fatalf("expected validation error for %s", tt.name)
			}

			validationErrs, ok := err.(validator.ValidationErrors)
			if !ok {
				t.Fatalf("expected ValidationErrors, got %T", err)
			}

			var fieldErr validator.FieldError
			for _, fe := range validationErrs {
				if fe.Field() == tt.field && fe.Tag() == tt.tag {
					fieldErr = fe
					break
				}
			}
			if fieldErr == nil {
				t.Fatalf("expected validation error for field %s with tag %s", tt.field, tt.tag)
			}

			if got := RuleMessage(fieldErr); got != tt.expected {
				t.Errorf("RuleMessage() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsNumericKind(t *testing.T) {
	tests := []struct {
		name     string
		kind     interface{}
		expected bool
	}{
		{"int", 0, true},
		{"int8", int8(0), true},
		{"int16", int16(0), true},
		{"int32", int32(0), true},
		{"int64", int64(0), true},
		{"float32", float32(0), true},
		{"float64", float64(0), true},
		{"string", "x", false},
		{"bool", false, false},
		{"struct", struct{}{}, false},
		{"slice", []int{}, false},
		{"map", map[string]int{}, false},
		{"pointer", new(int), false},
		{"func", func() {}, false},
		{"chan", make(chan int), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := reflect.TypeOf(tt.kind).Kind()
			if got := IsNumericKind(k); got != tt.expected {
				t.Errorf("IsNumericKind(%T) = %v, want %v", tt.kind, got, tt.expected)
			}
		})
	}
}
