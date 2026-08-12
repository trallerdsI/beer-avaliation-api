package model

import (
	"encoding/json"
	"testing"
)

func FuzzBeerJSON(f *testing.F) {
	f.Add(`{"name":"IPA","style":"Ale","alcohol":5.5,"taste":"Doce","aroma":"Floral","color":"Clara","body":"Leve","carbonation":"Baixa","finish":"Seco"}`)
	f.Add(`{}`)
	f.Add(`{"name":""}`)
	f.Add(`{"alcohol":999}`)
	f.Add(`{"comments":[{"id":"c1","text":"<script>alert(1)</script>","rating":5}]}`)
	f.Add(`{"media":[{"url":"http://example.com/img.jpg","type":"image/jpeg","size":1024}]}`)

	f.Fuzz(func(t *testing.T, data string) {
		var b Beer
		if err := json.Unmarshal([]byte(data), &b); err != nil {
			return
		}
		_, _ = json.Marshal(b)
	})
}

func FuzzCommentJSON(f *testing.F) {
	f.Add(`{"id":"c1","text":"Great beer!","rating":5,"likes":0,"likedBy":[],"createdBy":"user-1","createdAt":"2024-01-01T00:00:00Z"}`)
	f.Add(`{"id":"","text":"","rating":0}`)
	f.Add(`{"rating":5}`)

	f.Fuzz(func(t *testing.T, data string) {
		var c Comment
		if err := json.Unmarshal([]byte(data), &c); err != nil {
			return
		}
		_, _ = json.Marshal(c)
	})
}
