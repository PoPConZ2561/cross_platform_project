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
	result := model.BackupResult{Errors: make([]string, 0)}
	status := model.BackupProgress{Total: len(files)}
	if progress != nil {
		progress(status)
	}

	for _, sourcePath := range files {
		relative, err := filepath.Rel(source, sourcePath)
		if err != nil {
			return result, fmt.Errorf("resolve source image path: %w", err)
		}
		status.Current = relative
		metadata, analyzeErr := analyzeImage(sourcePath)
		if analyzeErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", relative, analyzeErr))
			status.Done++
			if progress != nil {
				progress(status)
			}
			continue
		}

		outputPath, hash, size, copyErr := copyImage(sourcePath, filepath.Join(destination, relative))
		if copyErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", relative, copyErr))
			status.Done++
			if progress != nil {
				progress(status)
			}
			continue
		}
		alreadyBackedUp, err := s.repo.ExistsHash(hash)
		if err != nil {
			if removeErr := os.Remove(outputPath); removeErr != nil {
				return result, fmt.Errorf("check duplicate image %s: %w (also failed to remove temporary copy: %v)", relative, err, removeErr)
			}
			return result, fmt.Errorf("check duplicate image %s: %w", relative, err)
		}
		if alreadyBackedUp {
			if err := os.Remove(outputPath); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("%s: remove duplicate temporary copy: %v", relative, err))
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
			FileName:     filepath.Base(sourcePath),
			OriginalName: filepath.Base(sourcePath),
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
				result.Errors = append(result.Errors, fmt.Sprintf("%s: remove incomplete backup copy: %v", relative, removeErr))
			}
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: save backup record: %v", relative, err))
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
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jpg", ".jpeg", ".png", ".gif":
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan source folder: %w", err)
	}
	return files, nil
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
