package repository

import (
	"cross_platform_project/model"
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDatabase(destFolder string) (*gorm.DB, error) {
	if err := os.MkdirAll(destFolder, 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	dbPath := filepath.Join(destFolder, "backup_history.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	// ใช้โหมด WAL เพื่อความปลอดภัยของฐานข้อมูลสำรองรูปภาพ
	if err := db.Exec("PRAGMA journal_mode = WAL;").Error; err != nil {
		return nil, fmt.Errorf("enable SQLite WAL mode: %w", err)
	}
	if err := db.Exec("PRAGMA synchronous = NORMAL;").Error; err != nil {
		return nil, fmt.Errorf("configure SQLite synchronous mode: %w", err)
	}

	err = db.AutoMigrate(&model.BackupRecord{}, &model.Tag{})
	if err != nil {
		return nil, err
	}
	return db, nil
}
