package service

import (
	"cross_platform_project/model"
	"cross_platform_project/repository"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type BackupService struct {
	repo *repository.BackupRepository
	mu   sync.Mutex
}

func NewBackupService(repo *repository.BackupRepository) *BackupService {
	return &BackupService{repo: repo}
}

func (s *BackupService) DeleteAll() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.repo.List()
	if err != nil {
		return 0, fmt.Errorf("list images before deleting all: %w", err)
	}
	paths := make(map[string]struct{}, len(records))
	var fileErrors []string
	for _, record := range records {
		if record.DestPath == "" {
			fileErrors = append(fileErrors, fmt.Sprintf("image %q has no backup path", record.FileName))
			continue
		}
		path, err := filepath.Abs(filepath.Clean(record.DestPath))
		if err != nil {
			fileErrors = append(fileErrors, fmt.Sprintf("%s: resolve backup path: %v", record.FileName, err))
			continue
		}
		key := strings.ToLower(filepath.Clean(path))
		if _, exists := paths[key]; exists {
			continue
		}
		paths[key] = struct{}{}

		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			fileErrors = append(fileErrors, fmt.Sprintf("%s: inspect backup file: %v", record.FileName, err))
			continue
		}
		if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			fileErrors = append(fileErrors, fmt.Sprintf("%s: backup path is not a file", record.FileName))
			continue
		}
		if err := os.Remove(path); err != nil {
			fileErrors = append(fileErrors, fmt.Sprintf("%s: delete backup file: %v", record.FileName, err))
		}
	}
	if len(fileErrors) > 0 {
		return 0, fmt.Errorf("some backup files could not be deleted; image records were kept so you can retry: %s", strings.Join(fileErrors, "; "))
	}

	deleted, err := s.repo.DeleteAll()
	if err != nil {
		return 0, fmt.Errorf("backup files were removed, but deleting image records failed: %w", err)
	}
	return len(deleted), nil
}

func (s *BackupService) Run(src, dst string, progress func(model.BackupProgress)) (model.BackupResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	source, destination, err := validateFolders(src, dst)
	if err != nil {
		return model.BackupResult{}, err
	}

	files, err := imageFiles(source)
	if err != nil {
		return model.BackupResult{}, err
	}
	relativePaths := make(map[string]string, len(files))
	for _, path := range files {
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return model.BackupResult{}, fmt.Errorf("resolve source image path: %w", err)
		}
		relativePaths[path] = relative
	}
	return s.runFiles(destination, files, relativePaths, progress)
}

func (s *BackupService) RunSelected(files []string, dst string, progress func(model.BackupProgress)) (model.BackupResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(files) == 0 {
		return model.BackupResult{}, errors.New("select at least one image")
	}
	destinationPath := strings.TrimSpace(dst)
	if destinationPath == "" {
		return model.BackupResult{}, errors.New("destination folder is required")
	}
	destination, err := filepath.Abs(filepath.Clean(destinationPath))
	if err != nil {
		return model.BackupResult{}, fmt.Errorf("resolve destination folder: %w", err)
	}
	destinationInfo, err := os.Stat(destination)
	if err != nil {
		return model.BackupResult{}, fmt.Errorf("open destination folder: %w", err)
	}
	if !destinationInfo.IsDir() {
		return model.BackupResult{}, errors.New("destination path is not a folder")
	}

	selected := make([]string, 0, len(files))
	relativePaths := make(map[string]string, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, path := range files {
		path = strings.TrimSpace(path)
		if path == "" {
			return model.BackupResult{}, errors.New("selected image path is invalid")
		}
		path, err = filepath.Abs(filepath.Clean(path))
		if err != nil {
			return model.BackupResult{}, fmt.Errorf("resolve selected image path: %w", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return model.BackupResult{}, fmt.Errorf("open selected image %q: %w", path, err)
		}
		if !info.Mode().IsRegular() || !isImageFile(path) {
			return model.BackupResult{}, fmt.Errorf("selected file is not a supported image: %s", path)
		}
		key := strings.ToLower(filepath.Clean(path))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, path)
		relativePaths[path] = filepath.Base(path)
	}
	return s.runFiles(destination, selected, relativePaths, progress)
}

func (s *BackupService) runFiles(destination string, files []string, relativePaths map[string]string, progress func(model.BackupProgress)) (model.BackupResult, error) {
	result := model.BackupResult{Errors: make([]string, 0)}
	status := model.BackupProgress{Total: len(files)}
	if progress != nil {
		progress(status)
	}

	for _, sourcePath := range files {
		fileName := filepath.Base(sourcePath)
		relative := relativePaths[sourcePath]
		status.Current = relative
		metadata, analyzeErr := analyzeImage(sourcePath)
		if analyzeErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", fileName, analyzeErr))
			status.Done++
			if progress != nil {
				progress(status)
			}
			continue
		}

		outputPath, hash, size, copyErr := copyImage(sourcePath, filepath.Join(destination, relative))
		if copyErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", fileName, copyErr))
			status.Done++
			if progress != nil {
				progress(status)
			}
			continue
		}
		alreadyBackedUp, err := s.repo.ExistsHash(hash)
		if err != nil {
			if removeErr := os.Remove(outputPath); removeErr != nil {
				return result, fmt.Errorf("check duplicate image %s: %w (also failed to remove temporary copy: %v)", fileName, err, removeErr)
			}
			return result, fmt.Errorf("check duplicate image %s: %w", fileName, err)
		}
		if alreadyBackedUp {
			if err := os.Remove(outputPath); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("%s: remove duplicate temporary copy: %v", fileName, err))
				status.Done++
				if progress != nil {
					progress(status)
				}
				continue
			}
			result.Skipped++
			status.Skipped++
			status.Done++
			if progress != nil {
				progress(status)
			}
			continue
		}

		record := model.BackupRecord{
			FileName:     fileName,
			OriginalName: fileName,
			SourcePath:   sourcePath,
			DestPath:     outputPath,
			FileSize:     size,
			FileHash:     hash,
			Description:  metadata.description,
			Status:       "completed",
		}
		record.Tags = make([]model.Tag, 0, len(metadata.tags))
		for _, tag := range metadata.tags {
			record.Tags = append(record.Tags, model.Tag{Name: tag})
		}
		if err := s.repo.Create(&record); err != nil {
			if removeErr := os.Remove(outputPath); removeErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: remove incomplete backup copy: %v", fileName, removeErr))
			}
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: save backup record: %v", fileName, err))
			status.Done++
			if progress != nil {
				progress(status)
			}
			continue
		}

		result.Copied++
		status.Done++
		if progress != nil {
			progress(status)
		}
	}
	return result, nil
}

func validateFolders(src, dst string) (string, string, error) {
	if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
		return "", "", errors.New("source and destination folders are required")
	}
	source, err := filepath.Abs(filepath.Clean(src))
	if err != nil {
		return "", "", fmt.Errorf("resolve source folder: %w", err)
	}
	destination, err := filepath.Abs(filepath.Clean(dst))
	if err != nil {
		return "", "", fmt.Errorf("resolve destination folder: %w", err)
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return "", "", fmt.Errorf("open source folder: %w", err)
	}
	if !sourceInfo.IsDir() {
		return "", "", errors.New("source path is not a folder")
	}
	destinationInfo, err := os.Stat(destination)
	if err != nil {
		return "", "", fmt.Errorf("open destination folder: %w", err)
	}
	if !destinationInfo.IsDir() {
		return "", "", errors.New("destination path is not a folder")
	}
	if pathsOverlap(source, destination) {
		return "", "", errors.New("source and destination folders must not contain one another")
	}
	return source, destination, nil
}

func pathsOverlap(first, second string) bool {
	if !strings.EqualFold(filepath.VolumeName(first), filepath.VolumeName(second)) {
		return false
	}
	firstRelative, err := filepath.Rel(first, second)
	if err != nil {
		return true
	}
	secondRelative, err := filepath.Rel(second, first)
	if err != nil {
		return true
	}
	return isWithinPath(firstRelative) || isWithinPath(secondRelative)
}

func isWithinPath(relative string) bool {
	return relative == "." || !filepath.IsAbs(relative) && !isParentPath(relative)
}

func isParentPath(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func imageFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if isImageFile(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan source folder: %w", err)
	}
	return files, nil
}

func isImageFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".gif":
		return true
	default:
		return false
	}
}

func copyImage(sourcePath, targetPath string) (string, string, int64, error) {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", "", 0, fmt.Errorf("create destination folder: %w", err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", "", 0, fmt.Errorf("open image: %w", err)
	}
	defer source.Close()

	temp, err := os.CreateTemp(filepath.Dir(targetPath), ".photo-backup-*")
	if err != nil {
		return "", "", 0, fmt.Errorf("create temporary backup: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(temp, hasher), source)
	if err != nil {
		temp.Close()
		return "", "", 0, fmt.Errorf("copy image: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return "", "", 0, fmt.Errorf("flush image backup: %w", err)
	}
	if err := temp.Close(); err != nil {
		return "", "", 0, fmt.Errorf("close image backup: %w", err)
	}

	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	outputPath, err := availablePath(targetPath, hash)
	if err != nil {
		return "", "", 0, err
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return "", "", 0, fmt.Errorf("save image backup: %w", err)
	}
	return outputPath, hash, size, nil
}

func availablePath(target, hash string) (string, error) {
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		return target, nil
	} else if err != nil {
		return "", fmt.Errorf("check destination file: %w", err)
	}

	ext := filepath.Ext(target)
	base := strings.TrimSuffix(target, ext)
	candidate := fmt.Sprintf("%s-%s%s", base, hash[:12], ext)
	for suffix := 1; ; suffix++ {
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("check destination file: %w", err)
		}
		candidate = fmt.Sprintf("%s-%s-%d%s", base, hash[:12], suffix, ext)
	}
}
