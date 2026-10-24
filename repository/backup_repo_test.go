package repository

import (
	"cross_platform_project/model"
	"path/filepath"
	"testing"
)

func TestBackupRepositoryMetadataLifecycle(t *testing.T) {
	db, err := InitDatabase(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	repo := NewBackupRepository(db)
	record := model.BackupRecord{
		FileName:    "trip.jpg",
		FileHash:    "hash-one",
		Description: "A 100% sunny beach",
		Tags:        []model.Tag{{Name: "beach"}, {Name: "summer"}},
	}
	if err := repo.Create(&record); err != nil {
		t.Fatalf("create backup record: %v", err)
	}

	records, err := repo.FindByTag("beach")
	if err != nil || len(records) != 1 || records[0].ID != record.ID {
		t.Fatalf("FindByTag() = %#v, %v; want record %d", records, err, record.ID)
	}
	records, err = repo.SearchDescription("100%")
	if err != nil || len(records) != 1 {
		t.Fatalf("SearchDescription() = %d records, %v; want one literal percent match", len(records), err)
	}
	records, err = repo.SearchDescription("sunny")
	if err != nil || len(records) != 1 {
		t.Fatalf("SearchDescription() = %d records, %v; want one match", len(records), err)
	}

	if err := repo.UpdateMetadata(record.ID, "Mountain hike", []string{"mountain", "summer", "mountain"}); err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	updated, err := repo.FindByID(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "Mountain hike" || len(updated.Tags) != 2 {
		t.Fatalf("updated metadata = description %q and %d tags; want description and two unique tags", updated.Description, len(updated.Tags))
	}
	tags, err := repo.TagCloud()
	if err != nil || len(tags) != 2 {
		t.Fatalf("TagCloud() = %#v, %v; want only the two used tags", tags, err)
	}

	deleted, err := repo.Delete(record.ID)
	if err != nil || deleted.ID != record.ID {
		t.Fatalf("Delete() = %#v, %v; want deleted record %d", deleted, err, record.ID)
	}
	tags, err = repo.TagCloud()
	if err != nil || len(tags) != 0 {
		t.Fatalf("TagCloud() after delete = %#v, %v; want no orphan tags", tags, err)
	}
}

func TestBackupRepositoryDeleteAllRemovesRecordsAndTags(t *testing.T) {
	db, err := InitDatabase(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	repo := NewBackupRepository(db)
	for _, record := range []model.BackupRecord{
		{FileName: "one.jpg", FileHash: "one", Tags: []model.Tag{{Name: "travel"}}},
		{FileName: "two.jpg", FileHash: "two", Tags: []model.Tag{{Name: "family"}}},
	} {
		if err := repo.Create(&record); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := repo.DeleteAll()
	if err != nil {
		t.Fatalf("DeleteAll(): %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("DeleteAll() returned %d records; want 2", len(deleted))
	}
	records, err := repo.List()
	if err != nil || len(records) != 0 {
		t.Fatalf("List() after DeleteAll() = %d records, %v; want none", len(records), err)
	}
	tags, err := repo.TagCloud()
	if err != nil || len(tags) != 0 {
		t.Fatalf("TagCloud() after DeleteAll() = %#v, %v; want no orphan tags", tags, err)
	}
}

func TestBackupRepositoryHistoryIsScopedToDestinationFolder(t *testing.T) {
	db, err := InitDatabase(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	repo := NewBackupRepository(db)
	root := t.TempDir()
	destinations := []string{
		filepath.Join(root, "backup"),
		filepath.Join(root, "backup", "camera"),
		filepath.Join(root, "backup-archive"),
	}
	for index, destination := range destinations {
		record := model.BackupRecord{
			FileName:    filepath.Base(destination) + ".jpg",
			DestPath:    filepath.Join(destination, "photo.jpg"),
			FileHash:    string(rune('a' + index)),
			Description: "shared description",
			Tags:        []model.Tag{{Name: "shared"}, {Name: "only-" + string(rune('a'+index))}},
		}
		if err := repo.Create(&record); err != nil {
			t.Fatal(err)
		}
	}

	records, err := repo.ListInFolder(destinations[0])
	if err != nil || len(records) != 2 {
		t.Fatalf("ListInFolder() = %d records, %v; want root and nested records only", len(records), err)
	}
	records, err = repo.FindByTagInFolder("shared", destinations[0])
	if err != nil || len(records) != 2 {
		t.Fatalf("FindByTagInFolder() = %d records, %v; want two scoped records", len(records), err)
	}
	records, err = repo.SearchDescriptionInFolder("shared", destinations[0])
	if err != nil || len(records) != 2 {
		t.Fatalf("SearchDescriptionInFolder() = %d records, %v; want two scoped records", len(records), err)
	}
	if _, err := repo.DeleteInFolder(records[0].ID, destinations[2]); err == nil {
		t.Fatal("DeleteInFolder() deleted a record outside the selected destination")
	}
	if err := repo.UpdateMetadataInFolder(records[0].ID, "changed outside folder", nil, destinations[2]); err == nil {
		t.Fatal("UpdateMetadataInFolder() changed a record outside the selected destination")
	}
	tags, err := repo.TagCloudInFolder(destinations[0])
	if err != nil || len(tags) != 3 {
		t.Fatalf("TagCloudInFolder() = %#v, %v; want shared and two local tags", tags, err)
	}

	deleted, err := repo.DeleteAllInFolder(destinations[0])
	if err != nil || len(deleted) != 2 {
		t.Fatalf("DeleteAllInFolder() = %d records, %v; want two", len(deleted), err)
	}
	remaining, err := repo.List()
	if err != nil || len(remaining) != 1 || remaining[0].DestPath != filepath.Join(destinations[2], "photo.jpg") {
		t.Fatalf("records after scoped delete = %#v, %v; sibling destination should remain", remaining, err)
	}
	tags, err = repo.TagCloud()
	if err != nil || len(tags) != 2 {
		t.Fatalf("global TagCloud() after scoped delete = %#v, %v; tags used in remaining folder should remain", tags, err)
	}
}
