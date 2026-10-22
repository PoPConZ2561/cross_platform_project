package main

import (
	"cross_platform_project/model"
	"cross_platform_project/repository"
	"cross_platform_project/service"
	"image"
	"image/color"
	"image/jpeg"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestImageHandlerServesOriginalAndThumbnail(t *testing.T) {
	root := t.TempDir()
	imagePath := filepath.Join(root, "photo.jpg")
	writeJPEG(t, imagePath)

	db, err := repository.InitDatabase(filepath.Join(root, "database"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	repo := repository.NewBackupRepository(db)
	record := model.BackupRecord{FileName: "photo.jpg", DestPath: imagePath, FileHash: "test-hash"}
	if err := repo.Create(&record); err != nil {
		t.Fatal(err)
	}
	app := &App{db: db, repo: repo, backup: service.NewBackupService(repo)}
	handler := app.ImageHandler()

	original := httptest.NewRecorder()
	handler.ServeHTTP(original, httptest.NewRequest("GET", "/image/1", nil))
	if original.Code != 200 || original.Body.Len() == 0 {
		t.Fatalf("original image response = status %d, %d bytes", original.Code, original.Body.Len())
	}

	thumbnail := httptest.NewRecorder()
	handler.ServeHTTP(thumbnail, httptest.NewRequest("GET", "/image/1?thumb=1", nil))
	if thumbnail.Code != 200 || thumbnail.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("thumbnail response = status %d, content type %q", thumbnail.Code, thumbnail.Header().Get("Content-Type"))
	}
	decoded, _, err := image.Decode(thumbnail.Body)
	if err != nil {
		t.Fatalf("decode returned thumbnail: %v", err)
	}
	if decoded.Bounds().Dx() != 12 || decoded.Bounds().Dy() != 8 {
		t.Fatalf("thumbnail dimensions = %v; want 12x8", decoded.Bounds())
	}
}

func writeJPEG(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	picture := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			picture.Set(x, y, color.RGBA{G: 255, A: 255})
		}
	}
	if err := jpeg.Encode(file, picture, nil); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
