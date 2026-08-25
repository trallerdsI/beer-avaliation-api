package boot

import (
	"os"
	"testing"
)

func TestIsTestingInNonLocal(t *testing.T) {
	tests := []struct {
		name    string
		testing string
		appEnv  string
		env     string
		expect  bool
	}{
		{name: "testing false", testing: "false", appEnv: "", env: "", expect: false},
		{name: "testing true local", testing: "true", appEnv: "local", env: "", expect: false},
		{name: "testing true development", testing: "true", appEnv: "development", env: "", expect: false},
		{name: "testing true staging", testing: "true", appEnv: "staging", env: "", expect: true},
		{name: "testing true production", testing: "true", appEnv: "production", env: "", expect: true},
		{name: "testing true staging via ENV", testing: "true", appEnv: "", env: "staging", expect: true},
		{name: "testing true production via ENV", testing: "true", appEnv: "", env: "production", expect: true},
		{name: "testing true unknown env", testing: "true", appEnv: "unknown", env: "", expect: false},
		{name: "testing true empty env falls back", testing: "true", appEnv: "", env: "", expect: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("TESTING")
			os.Unsetenv("APP_ENV")
			os.Unsetenv("ENV")

			if tt.testing != "" {
				os.Setenv("TESTING", tt.testing)
			}
			if tt.appEnv != "" {
				os.Setenv("APP_ENV", tt.appEnv)
			}
			if tt.env != "" {
				os.Setenv("ENV", tt.env)
			}

			got := IsTestingInNonLocal()
			if got != tt.expect {
				t.Errorf("IsTestingInNonLocal() = %v, want %v", got, tt.expect)
			}

			os.Unsetenv("TESTING")
			os.Unsetenv("APP_ENV")
			os.Unsetenv("ENV")
		})
	}
}
