package image

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// DeleteImageFiles 删除图片文件
func DeleteImageFiles(staticDir, imagePathsJSON string) error {
	if imagePathsJSON == "" || imagePathsJSON == "[]" {
		return nil
	}

	var paths []string
	if err := json.Unmarshal([]byte(imagePathsJSON), &paths); err != nil {
		log.Printf("Parse image_paths JSON failed: %v", err)
		return err
	}

	for _, path := range paths {
		fullPath := filepath.Join(staticDir, "..", path) // path 已包含 /static
		fullPath = filepath.Clean(fullPath)

		if err := os.Remove(fullPath); err != nil {
			if os.IsNotExist(err) {
				log.Printf("WARN: Image file not found: %s", fullPath)
			} else {
				log.Printf("ERROR: Failed to delete image: %s, error: %v", fullPath, err)
			}
		} else {
			log.Printf("Deleted image file: %s", fullPath)
		}
	}

	return nil
}

// RemoveEmptyDir 删除空目录
func RemoveEmptyDir(dirPath string) error {
	if dirPath == "" {
		return nil
	}

	// 读取目录内容
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 目录不存在，忽略
		}
		return err
	}

	// 如果目录为空，删除
	if len(entries) == 0 {
		if err := os.Remove(dirPath); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		log.Printf("Removed empty directory: %s", dirPath)
	}

	return nil
}

// GetImageDirPath 获取图片所属目录路径
func GetImageDirPath(staticDir string, sourceID int64) string {
	return filepath.Join(staticDir, "images", fmt.Sprintf("%d", sourceID))
}
