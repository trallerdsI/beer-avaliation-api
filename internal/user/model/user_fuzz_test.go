package model

import (
	"encoding/json"
	"testing"
)

func FuzzUserJSON(f *testing.F) {
	f.Add(`{"username":"testuser","email":"test@example.com","password":"password123"}`)
	f.Add(`{"username":"","email":"invalid","password":""}`)
	f.Add(`{"email":"test@example.com"}`)
	f.Add(`{"password":"short"}`)

	f.Fuzz(func(t *testing.T, data string) {
		var u User
		if err := json.Unmarshal([]byte(data), &u); err != nil {
			return
		}
		_, _ = json.Marshal(u)
	})
}
