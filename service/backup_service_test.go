package service

import (
	"cross_platform_project/model"
	"cross_platform_project/repository"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
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

	first, err := svc.Run(source, destination, func(progress model.BackupProgress) {
		if progress.Done == progress.Total && progress.Done > 0 {
			time.Sleep(20 * time.Millisecond)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Copied != 1 || first.Skipped != 0 || first.Failed != 0 {
		t.Fatalf("first backup result = %#v; want one copied image", first)
	}
	if first.DurationMillis < 10 {
		t.Fatalf("reported duration = %d ms; want it to include final file processing", first.DurationMillis)
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

func TestRunSelectedCopiesOneOrManyImages(t *testing.T) {
	root := t.TempDir()
	sourceOne := filepath.Join(root, "first", "photo.jpg")
	sourceTwo := filepath.Join(root, "second", "different.jpg")
	destination := filepath.Join(root, "destination")
	for _, folder := range []string{filepath.Dir(sourceOne), filepath.Dir(sourceTwo), destination} {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestJPEG(t, sourceOne)
	writeTestJPEGVariant(t, sourceTwo, 80)

	db, err := repository.InitDatabase(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	repo := repository.NewBackupRepository(db)
	svc := NewBackupService(repo)

	result, err := svc.RunSelected([]string{sourceOne}, destination, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Copied != 1 || result.Failed != 0 {
		t.Fatalf("single-image backup result = %#v; want one copy", result)
	}

	result, err = svc.RunSelected([]string{sourceOne, sourceTwo}, destination, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Copied != 1 || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("multi-image backup result = %#v; want one new image and one duplicate", result)
	}

	records, err := repo.List()
	if err != nil || len(records) != 2 {
		t.Fatalf("stored records = %d, %v; want two", len(records), err)
	}
}

func TestRunSelectedSkipsFilenameAlreadyInDestination(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "photo.jpg")
	destination := filepath.Join(root, "destination")
	for _, folder := range []string{filepath.Dir(source), destination} {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestJPEG(t, source)
	existingPath := filepath.Join(destination, "photo.jpg")
	writeTestJPEGVariant(t, existingPath, 80)
	original, err := os.ReadFile(existingPath)
	if err != nil {
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
	repo := repository.NewBackupRepository(db)

	result, err := NewBackupService(repo).RunSelected([]string{source}, destination, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Copied != 0 || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("backup result = %#v; want one skipped file", result)
	}
	current, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(original) {
		t.Fatal("existing destination file was modified")
	}
	records, err := repo.List()
	if err != nil || len(records) != 0 {
		t.Fatalf("stored records = %d, %v; want no new record", len(records), err)
	}
}

func TestRunSelectedSkipsFilenameAlreadyInDatabase(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "photo.jpg")
	destination := filepath.Join(root, "destination")
	archive := filepath.Join(destination, "archive")
	for _, folder := range []string{filepath.Dir(source), archive} {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestJPEG(t, source)
	existingRecordPath := filepath.Join(archive, "renamed.jpg")
	writeTestJPEGVariant(t, existingRecordPath, 80)

	db, err := repository.InitDatabase(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	repo := repository.NewBackupRepository(db)
	if err := repo.Create(&model.BackupRecord{
		FileName: "photo.jpg",
		DestPath: existingRecordPath,
		FileHash: "existing",
	}); err != nil {
		t.Fatal(err)
	}

	result, err := NewBackupService(repo).RunSelected([]string{source}, destination, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Copied != 0 || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("backup result = %#v; want one skipped file", result)
	}
	if _, err := os.Stat(filepath.Join(destination, "photo.jpg")); !os.IsNotExist(err) {
		t.Fatalf("file was copied despite matching database filename: %v", err)
	}
}

func TestRunCopiesSameImageToDifferentDestinations(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "trip.jpg")
	firstDestination := filepath.Join(root, "backup-one")
	secondDestination := filepath.Join(root, "backup-two")
	for _, folder := range []string{filepath.Dir(source), firstDestination, secondDestination} {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestJPEG(t, source)

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

	first, err := svc.RunSelected([]string{source}, firstDestination, nil)
	if err != nil || first.Copied != 1 || first.Skipped != 0 {
		t.Fatalf("backup to first destination = %#v, %v; want one copied", first, err)
	}
	second, err := svc.RunSelected([]string{source}, secondDestination, nil)
	if err != nil || second.Copied != 1 || second.Skipped != 0 {
		t.Fatalf("backup to second destination = %#v, %v; want one copied", second, err)
	}
	if _, err := os.Stat(filepath.Join(secondDestination, "trip.jpg")); err != nil {
		t.Fatalf("image was not copied to second destination: %v", err)
	}

	repeated, err := svc.RunSelected([]string{source}, secondDestination, nil)
	if err != nil || repeated.Copied != 0 || repeated.Skipped != 1 {
		t.Fatalf("repeated backup to second destination = %#v, %v; want one duplicate skipped", repeated, err)
	}
}

func TestDeleteAllRemovesBackupFilesAndRecords(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "destination")
	otherDestination := filepath.Join(root, "other-destination")
	for _, folder := range []string{destination, otherDestination} {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
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
	repo := repository.NewBackupRepository(db)
	for i, name := range []string{"one.jpg", "two.jpg"} {
		path := filepath.Join(destination, name)
		writeTestJPEGVariant(t, path, uint8(100+i))
		if err := repo.Create(&model.BackupRecord{
			FileName: name, DestPath: path, FileHash: name,
			Tags: []model.Tag{{Name: "test"}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	otherPath := filepath.Join(otherDestination, "keep.jpg")
	writeTestJPEGVariant(t, otherPath, 180)
	if err := repo.Create(&model.BackupRecord{
		FileName: "keep.jpg", DestPath: otherPath, FileHash: "keep",
		Tags: []model.Tag{{Name: "test"}},
	}); err != nil {
		t.Fatal(err)
	}

	deleted, err := NewBackupService(repo).DeleteAllInFolder(destination)
	if err != nil {
		t.Fatalf("DeleteAll(): %v", err)
	}
	if deleted != 2 {
		t.Fatalf("DeleteAllInFolder() returned %d; want 2", deleted)
	}
	for _, name := range []string{"one.jpg", "two.jpg"} {
		if _, err := os.Stat(filepath.Join(destination, name)); !os.IsNotExist(err) {
			t.Errorf("backup file %s still exists or returned unexpected error: %v", name, err)
		}
	}
	records, err := repo.List()
	if err != nil || len(records) != 1 || records[0].DestPath != otherPath {
		t.Fatalf("List() after scoped delete = %#v, %v; want only the other destination record", records, err)
	}
	if _, err := os.Stat(otherPath); err != nil {
		t.Fatalf("backup in another destination was deleted: %v", err)
	}
}

func writeTestJPEG(t *testing.T, path string) {
	t.Helper()
	writeTestJPEGVariant(t, path, 255)
}

func writeTestJPEGVariant(t *testing.T, path string, red uint8) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	picture := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			picture.Set(x, y, color.RGBA{R: red, A: 255})
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
