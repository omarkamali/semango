package ingest

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/omneity-labs/semango/internal/config"
)

// Crawl scans the filesystem. It sends file paths to filePathChan and
// a single error to errChan if the walk terminates due to an error or initial setup fails.
// It always closes filePathChan.
func Crawl(cfg config.FilesConfig, filePathChan chan<- string, errChan chan<- error) {
	defer close(filePathChan)

	slog.Info("Starting filesystem crawl...", "include", cfg.Include, "exclude", cfg.Exclude)
	rootDir, err := os.Getwd()
	if err != nil {
		slog.Error("Failed to get working directory for crawl", "error", err)
		select {
		case errChan <- err:
		default:
			slog.Warn("errChan full/blocked sending Getwd error")
		}
		return
	}
	slog.Debug("Crawling directory", "root", rootDir)

	walkErr := filepath.WalkDir(rootDir, func(absPath string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("Error accessing path during walk", "path", absPath, "error", err)
			return err // Propagate error, WalkDir might stop or skip based on this.
		}

		// Get path relative to rootDir for matching, as patterns are usually relative.
		relPath, err := filepath.Rel(rootDir, absPath)
		if err != nil {
			slog.Error("Failed to get relative path", "absPath", absPath, "rootDir", rootDir, "error", err)
			return err // Cannot proceed with this path if relative path fails
		}
		normalizedPath := filepath.ToSlash(relPath) // Use forward slashes for matching

		slog.Debug("WalkDir processing", "absPath", absPath, "relPath", normalizedPath, "isDir", d.IsDir())

		if d.IsDir() {
			if relPath == "." { // Skip processing for the root itself, just continue walk
				return nil
			}
			// Check if this directory should be excluded
			for _, excludePattern := range cfg.Exclude {
				patternToCheck := excludePattern
				if strings.HasSuffix(patternToCheck, "/**") {
					patternToCheck = strings.TrimSuffix(patternToCheck, "/**")
				}
				if matched, _ := doublestar.Match(patternToCheck, normalizedPath); matched {
					if excludePattern == patternToCheck || strings.HasSuffix(excludePattern, "/**") {
						slog.Debug("Excluding directory due to pattern", "dir_path", normalizedPath, "pattern", excludePattern)
						return filepath.SkipDir // Use filepath.SkipDir with WalkDir
					}
				}
			}
			return nil // Directory not excluded, continue walking
		}

		// It's a file. Check exclude patterns.
		for _, excludePattern := range cfg.Exclude {
			if matched, _ := doublestar.Match(excludePattern, normalizedPath); matched {
				slog.Debug("Excluding file due to pattern", "file_path", normalizedPath, "pattern", excludePattern)
				return nil
			}
		}

		// File is not excluded. Check include patterns.
		included := false
		if len(cfg.Include) == 0 {
			included = true
		} else {
			for _, includePattern := range cfg.Include {
				if matched, _ := doublestar.Match(includePattern, normalizedPath); matched {
					included = true
					break
				}
			}
		}

		if included {
			slog.Debug("Found matching file for processing", "file_path", normalizedPath)
			filePathChan <- normalizedPath // Send the relative path
		}
		return nil
	})

	if walkErr != nil {
		slog.Error("Filesystem walk ended with an error", "error", walkErr)
		select {
		case errChan <- walkErr:
		default:
			slog.Warn("errChan full/blocked sending walkErr from WalkDir")
		}
	} else {
		slog.Info("Filesystem walk completed successfully.")
	}
}
