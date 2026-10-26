package repository

import (
	"cross_platform_project/model"
	"errors"
	"os"
	"path/filepath"
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

func (r *BackupRepository) ExistsHashInFolder(hash, folder string) (bool, error) {
	folder, err := filepath.Abs(filepath.Clean(folder))
	if err != nil {
		return false, err
	}
	var paths []string
	if err := r.db.Model(&model.BackupRecord{}).
		Where("file_hash = ?", hash).
		Pluck("dest_path", &paths).Error; err != nil {
		return false, err
	}
	for _, path := range paths {
		if path == "" {
			continue
		}
		path, err = filepath.Abs(filepath.Clean(path))
		if err != nil {
			return false, err
		}
		relative, err := filepath.Rel(folder, path)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		if info.Mode().IsRegular() {
			return true, nil
		}
	}
	return false, nil
}

func (r *BackupRepository) ExistsFileNameInFolder(fileName, folder string) (bool, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return false, nil
	}
	query, err := recordsInFolder(r.db, folder)
	if err != nil {
		return false, err
	}
	records := make([]model.BackupRecord, 0)
	if err := query.Select("file_name", "dest_path").Find(&records).Error; err != nil {
		return false, err
	}
	for _, record := range records {
		if strings.EqualFold(record.FileName, fileName) || strings.EqualFold(filepath.Base(record.DestPath), fileName) {
			return true, nil
		}
	}
	return false, nil
}

func (r *BackupRepository) DuplicateIndexInFolder(folder string) (map[string]struct{}, map[string]struct{}, error) {
	query, err := recordsInFolder(r.db, folder)
	if err != nil {
		return nil, nil, err
	}
	records := make([]model.BackupRecord, 0)
	if err := query.Select("file_name", "dest_path", "file_hash").Find(&records).Error; err != nil {
		return nil, nil, err
	}

	names := make(map[string]struct{}, len(records))
	hashes := make(map[string]struct{}, len(records))
	for _, record := range records {
		if record.FileName != "" {
			names[strings.ToLower(record.FileName)] = struct{}{}
		}
		if record.DestPath != "" {
			names[strings.ToLower(filepath.Base(record.DestPath))] = struct{}{}
		}
		if record.FileHash == "" || record.DestPath == "" {
			continue
		}
		info, err := os.Stat(record.DestPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if info.Mode().IsRegular() {
			hashes[record.FileHash] = struct{}{}
		}
	}
	return names, hashes, nil
}

func (r *BackupRepository) List() ([]model.BackupRecord, error) {
	records := make([]model.BackupRecord, 0)
	err := r.db.Preload("Tags").Order("created_at DESC, id DESC").Find(&records).Error
	return records, err
}

func (r *BackupRepository) ListInFolder(folder string) ([]model.BackupRecord, error) {
	query, err := recordsInFolder(r.db, folder)
	if err != nil {
		return nil, err
	}
	records := make([]model.BackupRecord, 0)
	err = query.Preload("Tags").Order("created_at DESC, id DESC").Find(&records).Error
	return records, err
}

func (r *BackupRepository) BackupPathsInFolder(folder string) ([]model.BackupRecord, error) {
	query, err := recordsInFolder(r.db, folder)
	if err != nil {
		return nil, err
	}
	records := make([]model.BackupRecord, 0)
	err = query.Select("file_name", "dest_path").Find(&records).Error
	return records, err
}

// FindByIDsInFolder คืนเฉพาะรูปที่อยู่ในโฟลเดอร์ปลายทางนี้ ตามลำดับ id ที่ส่งมา
func (r *BackupRepository) FindByIDsInFolder(ids []uint, folder string) ([]model.BackupRecord, error) {
	query, err := recordsInFolder(r.db, folder)
	if err != nil {
		return nil, err
	}
	records := make([]model.BackupRecord, 0, len(ids))
	if len(ids) == 0 {
		return records, nil
	}
	if err := query.Where("id IN ?", ids).Find(&records).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]model.BackupRecord, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}
	ordered := make([]model.BackupRecord, 0, len(records))
	for _, id := range ids {
		if record, ok := byID[id]; ok {
			ordered = append(ordered, record)
			delete(byID, id)
		}
	}
	return ordered, nil
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

func (r *BackupRepository) FindByTagInFolder(name, folder string) ([]model.BackupRecord, error) {
	query, err := recordsInFolder(r.db, folder)
	if err != nil {
		return nil, err
	}
	records := make([]model.BackupRecord, 0)
	err = query.
		Joins("JOIN record_tags ON record_tags.backup_record_id = backup_records.id").
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

func (r *BackupRepository) SearchDescriptionInFolder(query, folder string) ([]model.BackupRecord, error) {
	recordsQuery, err := recordsInFolder(r.db, folder)
	if err != nil {
		return nil, err
	}
	records := make([]model.BackupRecord, 0)
	err = recordsQuery.
		Where("description LIKE ? ESCAPE '\\'", "%"+escapeLike(query)+"%").
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

func (r *BackupRepository) TagCloudInFolder(folder string) ([]model.TagCount, error) {
	condition, args, err := folderCondition(folder)
	if err != nil {
		return nil, err
	}
	tags := make([]model.TagCount, 0)
	err = r.db.Model(&model.Tag{}).
		Select("tags.name, COUNT(record_tags.backup_record_id) AS count").
		Joins("JOIN record_tags ON record_tags.tag_id = tags.id").
		Joins("JOIN backup_records ON backup_records.id = record_tags.backup_record_id").
		Where(condition, args...).
		Group("tags.id, tags.name").
		Order("tags.name").
		Scan(&tags).Error
	return tags, err
}

func (r *BackupRepository) UpdateMetadata(id uint, description string, names []string) error {
	return r.updateMetadata(id, description, names, "", nil)
}

func (r *BackupRepository) UpdateMetadataInFolder(id uint, description string, names []string, folder string) error {
	condition, args, err := folderCondition(folder)
	if err != nil {
		return err
	}
	return r.updateMetadata(id, description, names, condition, args)
}

func (r *BackupRepository) updateMetadata(id uint, description string, names []string, condition string, args []interface{}) error {
	normalized := normalizeTags(names)
	return r.db.Transaction(func(tx *gorm.DB) error {
		var record model.BackupRecord
		query := tx.Where("id = ?", id)
		if condition != "" {
			query = query.Where(condition, args...)
		}
		if err := query.First(&record).Error; err != nil {
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

func (r *BackupRepository) DeleteInFolder(id uint, folder string) (model.BackupRecord, error) {
	var record model.BackupRecord
	condition, args, err := folderCondition(folder)
	if err != nil {
		return record, err
	}
	err = r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Preload("Tags").
			Where("id = ?", id).
			Where(condition, args...).
			First(&record).Error; err != nil {
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

func (r *BackupRepository) DeleteAllInFolder(folder string) ([]model.BackupRecord, error) {
	records, err := r.ListInFolder(folder)
	if err != nil {
		return nil, err
	}
	condition, args, err := folderCondition(folder)
	if err != nil {
		return nil, err
	}
	err = r.db.Transaction(func(tx *gorm.DB) error {
		recordIDs := tx.Model(&model.BackupRecord{}).Select("id").Where(condition, args...)
		if err := tx.Exec("DELETE FROM record_tags WHERE backup_record_id IN (?)", recordIDs).Error; err != nil {
			return err
		}
		if err := tx.Where(condition, args...).Delete(&model.BackupRecord{}).Error; err != nil {
			return err
		}
		return removeUnusedTags(tx)
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func recordsInFolder(db *gorm.DB, folder string) (*gorm.DB, error) {
	condition, args, err := folderCondition(folder)
	if err != nil {
		return nil, err
	}
	return db.Model(&model.BackupRecord{}).Where(condition, args...), nil
}

func folderCondition(folder string) (string, []interface{}, error) {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return "", nil, errors.New("destination folder is required")
	}
	absolute, err := filepath.Abs(filepath.Clean(folder))
	if err != nil {
		return "", nil, err
	}
	prefix := absolute
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	condition := "dest_path = ? OR dest_path LIKE ? ESCAPE '\\'"
	return condition, []interface{}{absolute, escapeLike(prefix) + "%"}, nil
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
