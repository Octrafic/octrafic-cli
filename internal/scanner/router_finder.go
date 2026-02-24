package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RouterFile contains the path to a routing file and some metadata about it
type RouterFile struct {
	Path        string
	HitKeywords []string
}

// findRoutingFiles scans the project using filepath.WalkDir and identifies files that contain
// routing definitions based on common framework keywords.
func (s *Scanner) findRoutingFiles(framework *ProjectFramework, progressCallback func(string)) ([]RouterFile, error) {
	if progressCallback != nil {
		progressCallback("➔ Locating API routes...")
	}

	// Heuristics based on common frameworks
	var keywords []string
	switch strings.ToLower(framework.Language) {
	case "go":
		keywords = []string{
			"http.HandleFunc", "mux.Handle", "r.GET", "r.POST",
			"chi.NewRouter", "gin.Default", "fiber.New", "app.Get",
			"r.HandleFunc", "HandleFunc(",
		}
	case "python":
		keywords = []string{
			"@app.route", "@router.get", "@api.get", "path(", "re_path(", "def get(",
		}
	case "javascript", "typescript":
		keywords = []string{
			"app.get(", "app.post(", "router.get(", "app.use('/", "@Controller", "@Get(",
		}
	case "java", "kotlin":
		keywords = []string{
			"@RestController", "@RequestMapping", "@GetMapping", "class ",
		}
	case "php":
		keywords = []string{
			"Route::get", "Route::post", "$app->get(",
		}
	default: // Catch-all sensible defaults
		keywords = []string{
			"router.", ".get(", ".post(", "@get", "Route::",
		}
	}

	var routingFiles []RouterFile
	err := filepath.WalkDir(s.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(s.dir, path)
		if s.matcher != nil && s.matcher.IsIgnored(relPath, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// Skip files that are huge, binary, or clearly not logic files
		if isIgnorableExtension(path) {
			return nil
		}

		info, err := d.Info()
		if err != nil || info.Size() > 1024*1024*2 { // skip >2MB files for routing check
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)
		var hits []string

		for _, kw := range keywords {
			if strings.Contains(contentStr, kw) {
				hits = append(hits, kw)
			}
		}

		if len(hits) > 0 {
			routingFiles = append(routingFiles, RouterFile{
				Path:        relPath,
				HitKeywords: hits,
			})
			if progressCallback != nil {
				progressCallback(fmt.Sprintf("  ↳ Found routes in: %s", relPath))
			}
		}

		return nil
	})

	return routingFiles, err
}

func isIgnorableExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	ignorable := map[string]bool{
		".md": true, ".txt": true, ".log": true, ".csv": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".mp4": true, ".mp3": true, ".wav": true, ".pdf": true,
		".zip": true, ".tar": true, ".gz": true, ".exe": true,
		".dll": true, ".so": true, ".dylib": true, ".bin": true,
		".css": true, ".scss": true, ".html": true, ".svg": true,
	}
	return ignorable[ext]
}
