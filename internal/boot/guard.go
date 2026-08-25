package boot

import (
	"os"
	"strings"
)

func IsTestingInNonLocal() bool {
	if os.Getenv("TESTING") != "true" {
		return false
	}
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	}
	return env == "staging" || env == "production"
}
