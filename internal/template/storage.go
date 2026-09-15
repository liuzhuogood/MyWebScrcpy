package template

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Storage manages local persistence and memory caching of templates.
type Storage struct {
	baseDir   string
	mu        sync.RWMutex
	templates map[string]*Template
	images    map[string]image.Image
	variants  map[string][]image.Image

	watcher   *DirWatcher
	callbacks []func(serials []string)
	cbMu      sync.RWMutex
	closed    bool
}

// DefaultStorageDir keeps the template directory robust and writable when the
// service is launched by a supervisor without a valid or writable working directory.
func DefaultStorageDir() string {
	if dir := strings.TrimSpace(os.Getenv("TEMPLATES_DIR")); dir != "" {
		return dir
	}
	localDir := filepath.Join("data", "templates")
	if err := os.MkdirAll(localDir, 0755); err == nil {
		return localDir
	}
	if userConfigDir, err := os.UserConfigDir(); err == nil && userConfigDir != "" {
		return filepath.Join(userConfigDir, "mywebscrcpy", "templates")
	}
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "mywebscrcpy", "templates")
	}
	return filepath.Join(os.TempDir(), "mywebscrcpy-templates")
}

// NewStorage initializes a template storage engine using baseDir.
// If baseDir is empty, DefaultStorageDir() is used.
func NewStorage(baseDir string) (*Storage, error) {
	if baseDir == "" {
		baseDir = DefaultStorageDir()
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory (%s): %w", baseDir, err)
	}

	s := &Storage{
		baseDir:   baseDir,
		templates: make(map[string]*Template),
		images:    make(map[string]image.Image),
		variants:  make(map[string][]image.Image),
	}

	if err := s.loadExisting(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	w, err := NewDirWatcher(s.baseDir, 300*time.Millisecond, func() {
		_, _ = s.Reload()
	})
	if err == nil {
		if err := w.Start(); err == nil {
			s.watcher = w
		}
	}

	return s, nil
}

// Close closes the storage and any background watchers.
func (s *Storage) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	w := s.watcher
	s.watcher = nil
	s.mu.Unlock()

	if w != nil {
		return w.Close()
	}
	return nil
}

// OnChange registers a callback to be invoked when templates are modified or reloaded.
func (s *Storage) OnChange(fn func(serials []string)) {
	if fn == nil {
		return
	}
	s.cbMu.Lock()
	defer s.cbMu.Unlock()
	s.callbacks = append(s.callbacks, fn)
}

func (s *Storage) notifyChange(serials []string) {
	if len(serials) == 0 {
		return
	}
	unique := make(map[string]bool, len(serials))
	filtered := make([]string, 0, len(serials))
	for _, ser := range serials {
		if !unique[ser] {
			unique[ser] = true
			filtered = append(filtered, ser)
		}
	}
	if len(filtered) == 0 {
		return
	}

	s.cbMu.RLock()
	cbs := make([]func(serials []string), len(s.callbacks))
	copy(cbs, s.callbacks)
	s.cbMu.RUnlock()

	for _, cb := range cbs {
		go cb(filtered)
	}
}

// BaseDir returns the root directory where templates are stored.
func (s *Storage) BaseDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.baseDir
}

type diskTemplateEntry struct {
	tmpl     *Template
	img      image.Image
	variants []image.Image
	modTime  time.Time
}

func (s *Storage) readDiskTemplate(tmplDir string) (*diskTemplateEntry, error) {
	metaPath := filepath.Join(tmplDir, "meta.json")
	imgPath := filepath.Join(tmplDir, "template.png")

	metaStat, err := os.Stat(metaPath)
	if err != nil {
		return nil, err
	}
	imgStat, err := os.Stat(imgPath)
	if err != nil {
		return nil, err
	}

	maxMod := metaStat.ModTime()
	if imgStat.ModTime().After(maxMod) {
		maxMod = imgStat.ModTime()
	}

	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var tmpl Template
	if err := json.Unmarshal(metaBytes, &tmpl); err != nil {
		return nil, err
	}

	imgFile, err := os.Open(imgPath)
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(imgFile)
	_ = imgFile.Close()
	if err != nil {
		return nil, err
	}

	variants := []image.Image{img}
	variantDir := filepath.Join(tmplDir, "variants")
	if entries, readErr := os.ReadDir(variantDir); readErr == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".png" {
				continue
			}
			fPath := filepath.Join(variantDir, entry.Name())
			if fi, sErr := entry.Info(); sErr == nil && fi.ModTime().After(maxMod) {
				maxMod = fi.ModTime()
			}
			f, openErr := os.Open(fPath)
			if openErr != nil {
				continue
			}
			variant, decodeErr := png.Decode(f)
			_ = f.Close()
			if decodeErr == nil {
				variants = append(variants, variant)
			}
		}
	}
	tmpl.ImageCount = len(variants)

	if maxMod.After(tmpl.UpdatedAt) {
		tmpl.UpdatedAt = maxMod
	}

	return &diskTemplateEntry{
		tmpl:     &tmpl,
		img:      img,
		variants: variants,
		modTime:  maxMod,
	}, nil
}

func (s *Storage) readAllDiskTemplates() (map[string]*diskTemplateEntry, error) {
	serialEntries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*diskTemplateEntry)
	for _, sEntry := range serialEntries {
		if !sEntry.IsDir() {
			continue
		}
		serialPath := filepath.Join(s.baseDir, sEntry.Name())
		idEntries, err := os.ReadDir(serialPath)
		if err != nil {
			continue
		}

		for _, idEntry := range idEntries {
			if !idEntry.IsDir() {
				continue
			}
			tmplDir := filepath.Join(serialPath, idEntry.Name())
			entry, err := s.readDiskTemplate(tmplDir)
			if err != nil {
				continue
			}
			result[entry.tmpl.ID] = entry
		}
	}
	return result, nil
}

// loadExisting traverses baseDir and populates the in-memory cache.
func (s *Storage) loadExisting() error {
	diskEntries, err := s.readAllDiskTemplates()
	if err != nil {
		return err
	}
	for id, entry := range diskEntries {
		s.templates[id] = entry.tmpl
		s.images[id] = entry.img
		s.variants[id] = entry.variants
	}
	return nil
}

// Reload re-traverses the base directory, compares with in-memory templates,
// incrementally applies additions, updates, and deletions, and returns all affected serials.
func (s *Storage) Reload() ([]string, error) {
	diskEntries, err := s.readAllDiskTemplates()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	affected := make(map[string]bool)

	// 1. Clear templates that no longer exist on disk
	for id, memTmpl := range s.templates {
		if _, exists := diskEntries[id]; !exists {
			delete(s.templates, id)
			delete(s.images, id)
			delete(s.variants, id)
			affected[memTmpl.Serial] = true
		}
	}

	// 2. Add or update templates from disk
	for id, disk := range diskEntries {
		memTmpl, exists := s.templates[id]
		if !exists {
			s.templates[id] = disk.tmpl
			s.images[id] = disk.img
			s.variants[id] = disk.variants
			affected[disk.tmpl.Serial] = true
		} else if isTemplateModified(memTmpl, disk) {
			s.templates[id] = disk.tmpl
			s.images[id] = disk.img
			s.variants[id] = disk.variants
			affected[memTmpl.Serial] = true
			affected[disk.tmpl.Serial] = true
		}
	}
	s.mu.Unlock()

	var serials []string
	for ser := range affected {
		serials = append(serials, ser)
	}
	sort.Strings(serials)

	if len(serials) > 0 {
		s.notifyChange(serials)
	}

	return serials, nil
}

func isTemplateModified(mem *Template, disk *diskTemplateEntry) bool {
	if mem == nil || disk == nil || disk.tmpl == nil {
		return true
	}
	dt := disk.tmpl
	if mem.Name != dt.Name ||
		mem.Serial != dt.Serial ||
		mem.Threshold != dt.Threshold ||
		mem.Method != dt.Method ||
		mem.Grayscale != dt.Grayscale ||
		mem.Enabled != dt.Enabled ||
		mem.Width != dt.Width ||
		mem.Height != dt.Height ||
		mem.SceneWidth != dt.SceneWidth ||
		mem.SceneHeight != dt.SceneHeight ||
		mem.ImageCount != dt.ImageCount ||
		!mem.UpdatedAt.Equal(dt.UpdatedAt) {
		return true
	}
	if len(mem.Scales) != len(dt.Scales) {
		return true
	}
	for i := range mem.Scales {
		if mem.Scales[i] != dt.Scales[i] {
			return true
		}
	}
	if disk.modTime.After(mem.UpdatedAt) {
		return true
	}
	return false
}

// CreateTemplate stores a new template metadata and image.
func (s *Storage) CreateTemplate(req CreateTemplateRequest, img image.Image) (*Template, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidTemplate)
	}

	var err error
	if img == nil {
		if req.ImageBase64 == "" {
			return nil, fmt.Errorf("%w: template image or image_base64 is required", ErrInvalidImage)
		}
		img, err = decodeBase64Image(req.ImageBase64)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidImage, err)
		}
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("%w: zero dimensions", ErrInvalidImage)
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = generateID()
	}
	if err := validateSegment(id); err != nil {
		return nil, fmt.Errorf("%w: invalid id: %v", ErrInvalidTemplate, err)
	}

	serial := NormalizeSerial(req.Serial)
	if err := validateSerial(serial); err != nil {
		return nil, fmt.Errorf("%w: invalid serial: %v", ErrInvalidTemplate, err)
	}

	method := req.Method
	if method == "" {
		method = MethodTmCcoeffNormed
	}
	if method != MethodTmCcoeffNormed && method != MethodTmSqdiffNormed {
		return nil, fmt.Errorf("%w: unsupported method %s", ErrInvalidTemplate, method)
	}

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = DefaultThreshold
	}

	grayscale := true
	if req.Grayscale != nil {
		grayscale = *req.Grayscale
	}

	scales := req.Scales
	if len(scales) == 0 {
		scales = []float64{1.0}
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	now := time.Now().UTC()
	tmpl := &Template{
		ID:          id,
		Name:        req.Name,
		Serial:      serial,
		Threshold:   threshold,
		Method:      method,
		Grayscale:   grayscale,
		Scales:      scales,
		Enabled:     enabled,
		Width:       w,
		Height:      h,
		ImageCount:  1,
		SceneWidth:  req.SceneWidth,
		SceneHeight: req.SceneHeight,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.templates[id]; exists {
		return nil, ErrTemplateExists
	}

	targetDir := filepath.Join(s.baseDir, serialDir(serial), id)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	if err := writeTemplateFiles(targetDir, tmpl, img); err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, err
	}

	s.templates[id] = tmpl
	s.images[id] = img
	s.variants[id] = []image.Image{img}

	s.notifyChange([]string{tmpl.Serial})

	return tmpl.Clone(), nil
}

// GetTemplate returns the metadata of a template by ID.
func (s *Storage) GetTemplate(id string) (*Template, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tmpl, ok := s.templates[id]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	return tmpl.Clone(), nil
}

// GetTemplateImage returns the cached decoded image for a template.
func (s *Storage) GetTemplateImage(id string) (image.Image, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	img, ok := s.images[id]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	return img, nil
}

// GetTemplateImages returns every reference image for a template. The first is
// the original image retained for backwards-compatible downloads.
func (s *Storage) GetTemplateImages(id string) ([]image.Image, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	images, ok := s.variants[id]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	return append([]image.Image(nil), images...), nil
}

func (s *Storage) AddTemplateImage(id string, img image.Image) (*Template, error) {
	if img == nil || img.Bounds().Dx() < 2 || img.Bounds().Dy() < 2 {
		return nil, ErrInvalidImage
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tmpl, ok := s.templates[id]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	variantID := generateID()
	dir := filepath.Join(s.getTemplateDir(tmpl.Serial, id), "variants")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(dir, variantID+".png"))
	if err != nil {
		return nil, err
	}
	err = png.Encode(f, img)
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	s.variants[id] = append(s.variants[id], img)
	tmpl.ImageCount = len(s.variants[id])
	tmpl.UpdatedAt = time.Now().UTC()
	meta, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(s.getTemplateDir(tmpl.Serial, id), "meta.json"), meta, 0644); err != nil {
		return nil, err
	}
	s.notifyChange([]string{tmpl.Serial})
	return tmpl.Clone(), nil
}

// ListTemplates returns templates accessible to a given device serial, including global templates.
func (s *Storage) ListTemplates(serial string) []*Template {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetSerial := NormalizeSerial(serial)
	var list []*Template

	for _, tmpl := range s.templates {
		if tmpl.Serial == targetSerial || tmpl.Serial == GlobalSerial {
			list = append(list, tmpl.Clone())
		}
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID < list[j].ID
		}
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})

	return list
}

// ListTemplatesStrict returns only templates belonging strictly to the specified serial.
func (s *Storage) ListTemplatesStrict(serial string) []*Template {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetSerial := NormalizeSerial(serial)
	var list []*Template

	for _, tmpl := range s.templates {
		if tmpl.Serial == targetSerial {
			list = append(list, tmpl.Clone())
		}
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})

	return list
}

// ListAll returns all templates across all devices and global templates.
func (s *Storage) ListAll() []*Template {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*Template
	for _, tmpl := range s.templates {
		list = append(list, tmpl.Clone())
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID < list[j].ID
		}
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})

	return list
}

// UpdateTemplate modifies an existing template's metadata or image.
func (s *Storage) UpdateTemplate(id string, req UpdateTemplateRequest, newImg image.Image) (*Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.templates[id]
	if !ok {
		return nil, ErrTemplateNotFound
	}

	var err error
	if newImg == nil && req.ImageBase64 != nil && *req.ImageBase64 != "" {
		newImg, err = decodeBase64Image(*req.ImageBase64)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidImage, err)
		}
	}

	updated := existing.Clone()

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrInvalidTemplate)
		}
		updated.Name = name
	}

	oldSerial := existing.Serial
	newSerial := oldSerial
	if req.Serial != nil {
		newSerial = NormalizeSerial(*req.Serial)
		if err := validateSerial(newSerial); err != nil {
			return nil, fmt.Errorf("%w: invalid serial: %v", ErrInvalidTemplate, err)
		}
		updated.Serial = newSerial
	}

	if req.Threshold != nil {
		if *req.Threshold <= 0 {
			return nil, fmt.Errorf("%w: threshold must be > 0", ErrInvalidTemplate)
		}
		updated.Threshold = *req.Threshold
	}

	if req.Method != nil {
		if *req.Method != MethodTmCcoeffNormed && *req.Method != MethodTmSqdiffNormed {
			return nil, fmt.Errorf("%w: unsupported method %s", ErrInvalidTemplate, *req.Method)
		}
		updated.Method = *req.Method
	}

	if req.Grayscale != nil {
		updated.Grayscale = *req.Grayscale
	}

	if req.Scales != nil {
		if len(*req.Scales) == 0 {
			updated.Scales = []float64{1.0}
		} else {
			updated.Scales = *req.Scales
		}
	}

	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}

	if req.SceneWidth != nil {
		updated.SceneWidth = *req.SceneWidth
	}

	if req.SceneHeight != nil {
		updated.SceneHeight = *req.SceneHeight
	}

	cachedImg := s.images[id]
	if newImg != nil {
		bounds := newImg.Bounds()
		if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
			return nil, fmt.Errorf("%w: invalid image dimensions", ErrInvalidImage)
		}
		updated.Width = bounds.Dx()
		updated.Height = bounds.Dy()
		cachedImg = newImg
	}

	updated.UpdatedAt = time.Now().UTC()

	oldDir := s.getTemplateDir(oldSerial, id)
	newDir := filepath.Join(s.baseDir, serialDir(newSerial), id)

	if oldDir != newDir {
		if err := os.MkdirAll(filepath.Dir(newDir), 0755); err != nil {
			return nil, fmt.Errorf("failed to create new serial directory: %w", err)
		}
		if _, err := os.Stat(oldDir); err == nil {
			if err := os.Rename(oldDir, newDir); err != nil {
				return nil, fmt.Errorf("failed to move directory: %w", err)
			}
		}
	}

	if err := writeTemplateFiles(newDir, updated, cachedImg); err != nil {
		return nil, err
	}

	s.templates[id] = updated
	s.images[id] = cachedImg
	if newImg != nil {
		s.variants[id] = []image.Image{cachedImg}
		updated.ImageCount = 1
	}

	if oldSerial != newSerial {
		s.notifyChange([]string{oldSerial, newSerial})
	} else {
		s.notifyChange([]string{newSerial})
	}

	return updated.Clone(), nil
}

// DeleteTemplate deletes a template from disk and memory cache.
func (s *Storage) DeleteTemplate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmpl, ok := s.templates[id]
	if !ok {
		return ErrTemplateNotFound
	}

	dir := s.getTemplateDir(tmpl.Serial, id)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove directory: %w", err)
	}

	delete(s.templates, id)
	delete(s.images, id)
	delete(s.variants, id)

	s.notifyChange([]string{tmpl.Serial})

	return nil
}

// getTemplateDir returns the path to a template directory, checking existing path first for backward compatibility.
func (s *Storage) getTemplateDir(serial, id string) string {
	origDir := filepath.Join(s.baseDir, serial, id)
	if _, err := os.Stat(origDir); err == nil {
		return origDir
	}
	return filepath.Join(s.baseDir, serialDir(serial), id)
}

// serialDir returns a safe filesystem directory name for a given device serial.
func serialDir(serial string) string {
	if serial == GlobalSerial {
		return GlobalSerial
	}
	var b strings.Builder
	for _, r := range serial {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	res := b.String()
	if res == "" {
		return "unknown"
	}
	return res
}

// writeTemplateFiles saves template.png and meta.json to the target directory.
func writeTemplateFiles(dir string, tmpl *Template, img image.Image) error {
	metaPath := filepath.Join(dir, "meta.json")
	metaBytes, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize meta: %w", err)
	}
	if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
		return fmt.Errorf("failed to write meta.json: %w", err)
	}

	targetConfig := map[string]interface{}{
		"name":      tmpl.Name,
		"threshold": tmpl.Threshold,
		"grayscale": tmpl.Grayscale,
		"scales":    tmpl.Scales,
		"enabled":   tmpl.Enabled,
	}
	if targetBytes, err := json.MarshalIndent(targetConfig, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "target.json"), targetBytes, 0644)
	}

	imgPath := filepath.Join(dir, "template.png")
	imgFile, err := os.Create(imgPath)
	if err != nil {
		return fmt.Errorf("failed to create template.png: %w", err)
	}
	defer imgFile.Close()

	if err := png.Encode(imgFile, img); err != nil {
		return fmt.Errorf("failed to encode template.png: %w", err)
	}

	return nil
}

// decodeBase64Image parses a base64 encoded string (with optional data URL header) into image.Image.
func decodeBase64Image(data string) (image.Image, error) {
	idx := strings.Index(data, ",")
	raw := data
	if idx != -1 {
		raw = data[idx+1:]
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("base64 decode error: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(decoded))
	if err != nil {
		return nil, fmt.Errorf("image decode error: %w", err)
	}

	return img, nil
}

// validateSegment ensures an ID string is safe for filesystem paths.
func validateSegment(seg string) error {
	if seg == "" {
		return errors.New("empty segment")
	}
	clean := filepath.Clean(seg)
	if clean == "." || clean == ".." || strings.ContainsAny(clean, `/\\:*?"<>|`) {
		return fmt.Errorf("unsafe path characters in %q", seg)
	}
	return nil
}

// validateSerial ensures a device serial string is safe against path traversal.
func validateSerial(serial string) error {
	if serial == "" {
		return errors.New("empty serial")
	}
	clean := filepath.Clean(serial)
	if clean == "." || clean == ".." || strings.Contains(serial, "..") || strings.ContainsAny(serial, "/\\\x00") {
		return fmt.Errorf("unsafe path characters in serial %q", serial)
	}
	return nil
}

// generateID creates a unique identifier for a template.
func generateID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("tmpl_%d_%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

// FindTemplate finds a template by exact ID first, or by name (exact or case-insensitive)
// within the scope of the given serial (matching device-specific and global templates).
func (s *Storage) FindTemplate(serial, nameOrID string) (*Template, error) {
	nameOrID = strings.TrimSpace(nameOrID)
	if nameOrID == "" {
		return nil, ErrTemplateNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	normSerial := NormalizeSerial(serial)

	// 1. Direct ID lookup
	if tmpl, exists := s.templates[nameOrID]; exists {
		tSerial := NormalizeSerial(tmpl.Serial)
		if normSerial == GlobalSerial || tSerial == GlobalSerial || tSerial == normSerial {
			return tmpl.Clone(), nil
		}
	}

	// 2. Exact name match
	for _, tmpl := range s.templates {
		tSerial := NormalizeSerial(tmpl.Serial)
		if normSerial != GlobalSerial && tSerial != GlobalSerial && tSerial != normSerial {
			continue
		}
		if tmpl.Name == nameOrID {
			return tmpl.Clone(), nil
		}
	}

	// 3. Case-insensitive name match
	lower := strings.ToLower(nameOrID)
	for _, tmpl := range s.templates {
		tSerial := NormalizeSerial(tmpl.Serial)
		if normSerial != GlobalSerial && tSerial != GlobalSerial && tSerial != normSerial {
			continue
		}
		if strings.ToLower(tmpl.Name) == lower {
			return tmpl.Clone(), nil
		}
	}

	return nil, ErrTemplateNotFound
}
