package repository

import (
	"cross_platform_project/model"
	"strings"

	"gorm.io/gorm"
)

type BackupRepository struct {
	db *gorm.DB
}

func NewBackupRepository(db *gorm.DB) *BackupRepository {
	return &BackupRepository{db: db}
}

func (r *BackupRepository) Create(record *model.BackupRecord) error {
	names := make([]string, 0, len(record.Tags))
	for _, tag := range record.Tags {
		names = append(names, tag.Name)
	}
	normalized := normalizeTags(names)
	return r.db.Transaction(func(tx *gorm.DB) error {
		tags := make([]model.Tag, 0, len(normalized))
		for _, name := range normalized {
			tag := model.Tag{Name: name}
			if err := tx.Where("name = ?", name).FirstOrCreate(&tag).Error; err != nil {
				return err
			}
			tags = append(tags, tag)
		}

		record.Tags = nil
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		return tx.Model(record).Association("Tags").Replace(tags)
	})
}

func (r *BackupRepository) ExistsHash(hash string) (bool, error) {
	var count int64
	err := r.db.Model(&model.BackupRecord{}).Where("file_hash = ?", hash).Count(&count).Error
	return count > 0, err
}

func (r *BackupRepository) List() ([]model.BackupRecord, error) {
	records := make([]model.BackupRecord, 0)
	err := r.db.Preload("Tags").Order("created_at DESC, id DESC").Find(&records).Error
	return records, err
}

func (r *BackupRepository) FindByID(id uint) (model.BackupRecord, error) {
	var record model.BackupRecord
	err := r.db.Preload("Tags").First(&record, id).Error
	return record, err
}

func (r *BackupRepository) FindByTag(name string) ([]model.BackupRecord, error) {
	records := make([]model.BackupRecord, 0)
	err := r.db.Joins("JOIN record_tags ON record_tags.backup_record_id = backup_records.id").
		Joins("JOIN tags ON tags.id = record_tags.tag_id").
		Where("tags.name = ?", name).
		Preload("Tags").
		Order("backup_records.created_at DESC, backup_records.id DESC").
		Find(&records).Error
	return records, err
}

func (r *BackupRepository) SearchDescription(query string) ([]model.BackupRecord, error) {
	records := make([]model.BackupRecord, 0)
	err := r.db.Where("description LIKE ? ESCAPE '\\'", "%"+escapeLike(query)+"%").
		Preload("Tags").
		Order("created_at DESC, id DESC").
		Find(&records).Error
	return records, err
}

func (r *BackupRepository) TagCloud() ([]model.TagCount, error) {
	tags := make([]model.TagCount, 0)
	err := r.db.Model(&model.Tag{}).
		Select("tags.name, COUNT(record_tags.backup_record_id) AS count").
		Joins("LEFT JOIN record_tags ON record_tags.tag_id = tags.id").
		Group("tags.id, tags.name").
		Having("COUNT(record_tags.backup_record_id) > 0").
		Order("tags.name").
		Scan(&tags).Error
	return tags, err
}

func (r *BackupRepository) UpdateMetadata(id uint, description string, names []string) error {
	normalized := normalizeTags(names)
	return r.db.Transaction(func(tx *gorm.DB) error {
		var record model.BackupRecord
		if err := tx.First(&record, id).Error; err != nil {
			return err
		}

		tags := make([]model.Tag, 0, len(normalized))
		for _, name := range normalized {
			tag := model.Tag{Name: name}
			if err := tx.Where("name = ?", name).FirstOrCreate(&tag).Error; err != nil {
				return err
			}
			tags = append(tags, tag)
		}

		if err := tx.Model(&record).Update("description", strings.TrimSpace(description)).Error; err != nil {
			return err
		}
		if err := tx.Model(&record).Association("Tags").Replace(tags); err != nil {
			return err
		}
		return removeUnusedTags(tx)
	})
}

func (r *BackupRepository) Delete(id uint) (model.BackupRecord, error) {
	var record model.BackupRecord
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Preload("Tags").First(&record, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&record).Association("Tags").Clear(); err != nil {
			return err
		}
		if err := tx.Delete(&record).Error; err != nil {
			return err
		}
		return removeUnusedTags(tx)
	})
	return record, err
}

func (r *BackupRepository) DeleteAll() ([]model.BackupRecord, error) {
	records, err := r.List()
	if err != nil {
		return nil, err
	}
	err = r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM record_tags").Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&model.BackupRecord{}).Error; err != nil {
			return err
		}
		return tx.Where("1 = 1").Delete(&model.Tag{}).Error
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func removeUnusedTags(tx *gorm.DB) error {
	return tx.Where("NOT EXISTS (?)",
		tx.Model(&model.BackupRecord{}).
			Select("1").
			Joins("JOIN record_tags ON record_tags.backup_record_id = backup_records.id").
			Where("record_tags.tag_id = tags.id"),
	).Delete(&model.Tag{}).Error
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}

func normalizeTags(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	return normalized
}
