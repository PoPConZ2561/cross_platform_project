package service

import (
	"cross_platform_project/repository"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPathsOverlapAcrossWindowsDrives(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows drive-letter paths are only meaningful on Windows")
	}
	if pathsOverlap(`D:\`, `C:\Users\phanu\crossplatform\gallery`) {
		t.Fatal("paths on different drives must not be considered overlapping")
	}
}

func TestPathsOverlapSiblings(t *testing.T) {
	first := filepath.Join(string(filepath.Separator), "photos", "source")
	second := filepath.Join(string(filepath.Separator), "photos", "backup")
	if pathsOverlap(first, second) {
		t.Fatalf("sibling directories %q and %q must not overlap", first, second)
	}
}

func TestRunCopiesAndSkipsDuplicateImages(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(filepath.Join(source, "camera"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(source, "camera", "trip.jpg")
	writeTestJPEG(t, imagePath)

	db, err := repository.InitDatabase(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	svc := NewBackupService(repository.NewBackupRepository(db))

	first, err := svc.Run(source, destination, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Copied != 1 || first.Skipped != 0 || first.Failed != 0 {
		t.Fatalf("first backup result = %#v; want one copied image", first)
	}
	if _, err := os.Stat(filepath.Join(destination, "camera", "trip.jpg")); err != nil {
		t.Fatalf("image was not copied preserving its folder: %v", err)
	}

	second, err := svc.Run(source, destination, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.Copied != 0 || second.Skipped != 1 || second.Failed != 0 {
		t.Fatalf("second backup result = %#v; want one duplicate skipped", second)
	}
	items, err := repository.NewBackupRepository(db).List()
	if err != nil || len(items) != 1 {
		t.Fatalf("stored records = %d, %v; want exactly one", len(items), err)
	}
}

func TestRunRejectsOverlappingFolders(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "backup")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := repository.InitDatabase(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	_, err = NewBackupService(repository.NewBackupRepository(db)).Run(root, destination, nil)
	if err == nil {
		t.Fatal("Run() accepted a destination inside the source tree")
	}
}

func writeTestJPEG(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	picture := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			picture.Set(x, y, color.RGBA{R: 255, A: 255})
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
