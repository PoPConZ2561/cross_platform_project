package model

import "time"

type BackupRecord struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	FileName     string    `json:"file_name"`
	OriginalName string    `json:"original_name"`
	SourcePath   string    `json:"source_path"`
	DestPath     string    `json:"dest_path"`
	FileSize     int64     `json:"file_size"`
	FileHash     string    `json:"file_hash"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	Tags         []Tag     `gorm:"many2many:record_tags;" json:"tags"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Tag struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"uniqueIndex" json:"name"`
}
