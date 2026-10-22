package repository

import (
	"cross_platform_project/model"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDatabase(destFolder string) (*gorm.DB, error) {
	dbPath := filepath.Join(destFolder, "backup_history.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	// ใช้โหมด WAL เพื่อความปลอดภัยของฐานข้อมูลสำรองรูปภาพ
	db.Exec("PRAGMA journal_mode = WAL;")
	db.Exec("PRAGMA synchronous = NORMAL;")

	err = db.AutoMigrate(&model.BackupRecord{}, &model.Tag{})
	if err != nil {
		return nil, err
	}
	return db, nil
}
