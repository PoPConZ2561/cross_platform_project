package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cross_platform_project/model"
	"cross_platform_project/repository"
	"cross_platform_project/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

type App struct {
	ctx     context.Context
	db      *gorm.DB
	repo    *repository.BackupRepository
	backup  *service.BackupService
	initErr error
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	configDir, err := os.UserConfigDir()
	if err != nil {
		a.initErr = fmt.Errorf("find application data folder: %w", err)
		log.Printf("Photo Backup startup failed: %v", a.initErr)
		return
	}
	dataDir := filepath.Join(configDir, "PhotoBackup")
	a.db, err = repository.InitDatabase(dataDir)
	if err != nil {
		a.initErr = fmt.Errorf("initialize photo database: %w", err)
		log.Printf("Photo Backup startup failed: %v", a.initErr)
		return
	}
	a.repo = repository.NewBackupRepository(a.db)
	a.backup = service.NewBackupService(a.repo)
}

func (a *App) shutdown(context.Context) {
	if a.db == nil {
		return
	}
	sqlDB, err := a.db.DB()
	if err != nil {
		log.Printf("get photo database connection for shutdown: %v", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		log.Printf("close photo database: %v", err)
	}
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) SelectFolder(title string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("application is not ready")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title})
}

func (a *App) StartBackup(src, dst string) (model.BackupResult, error) {
	if err := a.ready(); err != nil {
		return model.BackupResult{}, err
	}
	return a.backup.Run(src, dst, func(progress model.BackupProgress) {
		runtime.EventsEmit(a.ctx, "backup:progress", progress)
	})
}

func (a *App) ListImages() ([]model.PhotoItem, error) {
	records, err := a.listRecords()
	if err != nil {
		return nil, err
	}
	return photoItems(records), nil
}

func (a *App) SearchByTag(tag string) ([]model.PhotoItem, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	records, err := a.repo.FindByTag(strings.TrimSpace(tag))
	if err != nil {
		return nil, fmt.Errorf("search images by tag: %w", err)
	}
	return photoItems(records), nil
}

func (a *App) SearchByDescription(query string) ([]model.PhotoItem, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []model.PhotoItem{}, nil
	}
	records, err := a.repo.SearchDescription(query)
	if err != nil {
		return nil, fmt.Errorf("search image descriptions: %w", err)
	}
	return photoItems(records), nil
}

func (a *App) TagCloud() ([]model.TagCount, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	tags, err := a.repo.TagCloud()
	if err != nil {
		return nil, fmt.Errorf("load image tags: %w", err)
	}
	return tags, nil
}

func (a *App) UpdateImage(id uint, description string, tags []string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == 0 {
		return errors.New("image id must be greater than zero")
	}
	if err := a.repo.UpdateMetadata(id, description, tags); err != nil {
		return fmt.Errorf("update image metadata: %w", err)
	}
	return nil
}

func (a *App) DeleteImage(id uint) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == 0 {
		return errors.New("image id must be greater than zero")
	}
	record, err := a.repo.Delete(id)
	if err != nil {
		return fmt.Errorf("delete image record: %w", err)
	}
	if err := os.Remove(record.DestPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("image record deleted but backup file could not be removed: %w", err)
	}
	return nil
}

func (a *App) ImageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := a.ready(); err != nil {
			http.Error(w, "image service unavailable", http.StatusServiceUnavailable)
			return
		}
		idText := strings.TrimPrefix(request.URL.Path, "/image/")
		id, err := strconv.ParseUint(idText, 10, 32)
		if err != nil || id == 0 || strings.Contains(idText, "/") {
			http.NotFound(w, request)
			return
		}
		record, err := a.repo.FindByID(uint(id))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				http.NotFound(w, request)
				return
			}
			log.Printf("load image %d for serving: %v", id, err)
			http.Error(w, "image service failed", http.StatusInternalServerError)
			return
		}
		if request.URL.Query().Get("thumb") == "1" {
			a.serveThumbnail(w, record.DestPath)
			return
		}
		http.ServeFile(w, request, record.DestPath)
	})
}

func (a *App) ImageMiddleware() func(http.Handler) http.Handler {
	imageHandler := a.ImageHandler()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/image" || strings.HasPrefix(request.URL.Path, "/image/") {
				imageHandler.ServeHTTP(w, request)
				return
			}
			next.ServeHTTP(w, request)
		})
	}
}

func (a *App) serveThumbnail(w http.ResponseWriter, path string) {
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "image file unavailable", http.StatusNotFound)
		return
	}
	defer file.Close()
	source, _, err := image.Decode(file)
	if err != nil {
		http.Error(w, "image could not be decoded", http.StatusInternalServerError)
		return
	}
	bounds := source.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		http.Error(w, "image has invalid dimensions", http.StatusInternalServerError)
		return
	}
	scale := min(400.0/float64(bounds.Dx()), 300.0/float64(bounds.Dy()), 1.0)
	width := max(1, int(float64(bounds.Dx())*scale))
	height := max(1, int(float64(bounds.Dy())*scale))
	thumbnail := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(thumbnail, thumbnail.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sx := bounds.Min.X + x*bounds.Dx()/width
			sy := bounds.Min.Y + y*bounds.Dy()/height
			thumbnail.Set(x, y, source.At(sx, sy))
		}
	}
	w.Header().Set("Content-Type", "image/jpeg")
	if err := jpeg.Encode(w, thumbnail, &jpeg.Options{Quality: 82}); err != nil {
		log.Printf("encode image thumbnail: %v", err)
	}
}

func (a *App) ready() error {
	if a.initErr != nil {
		return a.initErr
	}
	if a.repo == nil || a.backup == nil {
		return errors.New("application database is not initialized")
	}
	return nil
}

func (a *App) listRecords() ([]model.BackupRecord, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	records, err := a.repo.List()
	if err != nil {
		return nil, fmt.Errorf("list backed-up images: %w", err)
	}
	return records, nil
}

func photoItems(records []model.BackupRecord) []model.PhotoItem {
	items := make([]model.PhotoItem, 0, len(records))
	for _, record := range records {
		tags := make([]string, 0, len(record.Tags))
		for _, tag := range record.Tags {
			tags = append(tags, tag.Name)
		}
		items = append(items, model.PhotoItem{
			ID:          record.ID,
			FileName:    record.FileName,
			Description: record.Description,
			Tags:        tags,
			CreatedAt:   record.CreatedAt,
		})
	}
	return items
}
