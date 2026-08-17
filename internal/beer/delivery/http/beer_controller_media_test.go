package http

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/storage"
	"github.com/stretchr/testify/mock"
)

// fakeUploader é um storage.Uploader em memória para testes (sem rede).
type fakeUploader struct {
	got map[string][]byte
}

func (f *fakeUploader) Upload(_ context.Context, name, ct string, data []byte) (string, error) {
	if f.got == nil {
		f.got = map[string][]byte{}
	}
	f.got[name] = data
	return "https://storage.test/" + name, nil
}

func newTestUploadController(u *MockBeerUsecase, up storage.Uploader) *BeerController {
	return &BeerController{usecase: u, logger: mockLogger, uploader: up}
}

func encodeTestJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodeTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUploadBeerMedia_AcceptsValidJPEG(t *testing.T) {
	u := new(MockBeerUsecase)
	u.On("GetByID", mock.Anything, "beer1").Return(model.Beer{ID: "beer1", CreatedBy: "u1"}, nil)
	u.On("AddMedia", mock.Anything, "beer1", mock.Anything).Return([]model.MediaItem{
		{URL: "https://storage.test/beers/x.jpg", Type: "image/jpeg", Size: 4},
	}, nil)

	up := &fakeUploader{}
	c := newTestUploadController(u, up)

	jpegData := encodeTestJPEG(t, 100, 100)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", "photo.jpg")
	part.Write(jpegData)
	mw.Close()

	r := httptest.NewRequest(http.MethodPost, "/api/v1/beers/beer1/media", body)
	r.SetPathValue("id", "beer1")
	r = r.WithContext(middleware.WithUserID(r.Context(), "u1", "user"))
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()

	c.UploadBeerMedia(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp []model.MediaItem
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 1 || resp[0].Type != "image/jpeg" {
		t.Fatalf("media inesperada: %+v", resp)
	}
	if len(up.got) != 1 {
		t.Fatalf("uploader não recebeu o ficheiro")
	}
}

func TestUploadBeerMedia_RejectsNonImage(t *testing.T) {
	u := new(MockBeerUsecase)
	up := &fakeUploader{}
	c := newTestUploadController(u, up)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", "x.txt")
	part.Write([]byte("nao e imagem"))
	mw.Close()

	r := httptest.NewRequest(http.MethodPost, "/api/v1/beers/beer1/media", body)
	r.SetPathValue("id", "beer1")
	r = r.WithContext(middleware.WithUserID(r.Context(), "u1", "user"))
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()

	c.UploadBeerMedia(w, r)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, queria 415, body = %s", w.Code, w.Body.String())
	}
	if len(up.got) != 0 {
		t.Fatal("uploader não devia receber ficheiro inválido")
	}
}

func TestUploadBeerMedia_RejectsOversizedImage(t *testing.T) {
	u := new(MockBeerUsecase)
	up := &fakeUploader{}
	c := newTestUploadController(u, up)

	jpegData := encodeTestJPEG(t, 5000, 5000)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", "huge.jpg")
	part.Write(jpegData)
	mw.Close()

	r := httptest.NewRequest(http.MethodPost, "/api/v1/beers/beer1/media", body)
	r.SetPathValue("id", "beer1")
	r = r.WithContext(middleware.WithUserID(r.Context(), "u1", "user"))
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()

	c.UploadBeerMedia(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, queria 422, body = %s", w.Code, w.Body.String())
	}
	if len(up.got) != 0 {
		t.Fatal("uploader não devia receber ficheiro com dimensões excessivas")
	}
}

func TestUploadBeerMedia_AcceptsValidPNG(t *testing.T) {
	u := new(MockBeerUsecase)
	u.On("GetByID", mock.Anything, "beer1").Return(model.Beer{ID: "beer1", CreatedBy: "u1"}, nil)
	u.On("AddMedia", mock.Anything, "beer1", mock.Anything).Return([]model.MediaItem{
		{URL: "https://storage.test/beers/x.png", Type: "image/png", Size: 4},
	}, nil)

	up := &fakeUploader{}
	c := newTestUploadController(u, up)

	pngData := encodeTestPNG(t, 100, 100)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", "photo.png")
	part.Write(pngData)
	mw.Close()

	r := httptest.NewRequest(http.MethodPost, "/api/v1/beers/beer1/media", body)
	r.SetPathValue("id", "beer1")
	r = r.WithContext(middleware.WithUserID(r.Context(), "u1", "user"))
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()

	c.UploadBeerMedia(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp []model.MediaItem
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 1 || resp[0].Type != "image/png" {
		t.Fatalf("media inesperada: %+v", resp)
	}
	if len(up.got) != 1 {
		t.Fatalf("uploader não recebeu o ficheiro")
	}
}
