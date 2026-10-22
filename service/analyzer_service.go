package service

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

type imageMetadata struct {
	description string
	tags        []string
	width       int
	height      int
}

func analyzeImage(path string) (imageMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return imageMetadata{}, err
	}
	defer file.Close()

	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return imageMetadata{}, fmt.Errorf("read image metadata: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return imageMetadata{}, fmt.Errorf("image has invalid dimensions: %d x %d", config.Width, config.Height)
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "jpeg" {
		ext = "jpg"
	}
	tags := []string{ext}
	orientation := "แนวนอน"
	if config.Width == config.Height {
		orientation = "สี่เหลี่ยม"
	} else if config.Width < config.Height {
		orientation = "แนวตั้ง"
	}
	tags = append(tags, orientation)

	description := fmt.Sprintf("ภาพ %s ขนาด %d x %d พิกเซล", filepath.Base(path), config.Width, config.Height)
	if strings.ToLower(format) != ext && !(format == "jpeg" && ext == "jpg") {
		description += fmt.Sprintf(" (รูปแบบ %s)", strings.ToUpper(format))
	}
	return imageMetadata{description: description, tags: tags, width: config.Width, height: config.Height}, nil
}
