package validation

import (
	"reflect"

	"github.com/go-playground/validator/v10"
)

// FieldLabel traduz o nome do campo Go para um rótulo legível em PT.
func FieldLabel(field string) string { //nosec G101
	labels := map[string]string{
		"Name":        "nome",
		"Style":       "estilo",
		"Description": "descrição",
		"ImageUrl":    "imagem",
		"Alcohol":     "teor alcoólico",
		"Taste":       "sabor",
		"Aroma":       "aroma",
		"Color":       "cor",
		"Body":        "corpo",
		"Carbonation": "carbonatação",
		"Finish":      "finalização",
		"Text":        "texto",
		"Rating":      "avaliação",
		"Comments":    "comentários",
		"Username":    "nome de utilizador",
		"Email":       "email",
		"Password":    "palavra-passe",
		"Role":        "perfil",
	}
	if l, ok := labels[field]; ok {
		return l
	}
	return field
}

// RuleMessage gera uma frase clara para a regra violada.
func RuleMessage(fe validator.FieldError) string {
	param := fe.Param()
	switch fe.Tag() {
	case "required":
		return "é obrigatório"
	case "min":
		if IsNumericKind(fe.Kind()) {
			return "deve ser no mínimo " + param
		}
		return "deve ter pelo menos " + param + " caracteres"
	case "max":
		if IsNumericKind(fe.Kind()) {
			return "deve ser no máximo " + param
		}
		return "deve ter no máximo " + param + " caracteres"
	case "https_url":
		return "deve ser um URL https válido"
	case "email":
		return "deve ser um email válido"
	default:
		return "valor inválido"
	}
}

// IsNumericKind indica se o kind do campo é inteiro ou float.
func IsNumericKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}
