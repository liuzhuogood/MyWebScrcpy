package template

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DirWatcher monitors a directory tree for file system changes and notifies with debouncing.
type DirWatcher struct {
	baseDir   string
	debounce  time.Duration
	onChange  func()
	watcher   *fsnotify.Watcher
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
	watched   map[string]struct{}
	wg        sync.WaitGroup
}

// NewDirWatcher creates a new DirWatcher for baseDir.
// If debounce is <= 0, a default of 300ms is used.
func NewDirWatcher(baseDir string, debounce time.Duration, onChange func()) (*DirWatcher, error) {
	if baseDir == "" {
		return nil, errors.New("baseDir is required")
	}
	if debounce <= 0 {
		debounce = 300 * time.Millisecond
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &DirWatcher{
		baseDir:  baseDir,
		debounce: debounce,
		onChange: onChange,
		watcher:  fsWatcher,
		done:     make(chan struct{}),
		watched:  make(map[string]struct{}),
	}, nil
}

// Start begins recursively watching baseDir and processing fs events.
func (w *DirWatcher) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.watchRecursively(w.baseDir); err != nil {
		_ = w.watcher.Close()
		return err
	}

	w.wg.Add(1)
	go w.eventLoop()
	return nil
}

// Close stops the watcher and frees all resources.
func (w *DirWatcher) Close() error {
	var err error
	w.closeOnce.Do(func() {
		close(w.done)
		err = w.watcher.Close()
		w.wg.Wait()
	})
	return err
}

func (w *DirWatcher) watchRecursively(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if _, exists := w.watched[path]; !exists {
				if addErr := w.watcher.Add(path); addErr == nil {
					w.watched[path] = struct{}{}
				}
			}
		}
		return nil
	})
}

func (w *DirWatcher) addWatch(dir string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.watchRecursively(dir)
}

func (w *DirWatcher) removeWatch(dir string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.watched, dir)
	_ = w.watcher.Remove(dir)
}

func (w *DirWatcher) eventLoop() {
	defer w.wg.Done()

	var timer *time.Timer
	var timerCh <-chan time.Time

	stopTimer := func() {
		if timer != nil {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer = nil
			timerCh = nil
		}
	}
	defer stopTimer()

	for {
		select {
		case <-w.done:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Handle dynamic directory additions
			if event.Op&fsnotify.Create != 0 {
				if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
					w.addWatch(event.Name)
				}
			} else if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				w.removeWatch(event.Name)
			}

			// Ignore irrelevant chmod-only events
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			// Reset debounce timer
			if timer == nil {
				timer = time.NewTimer(w.debounce)
				timerCh = timer.C
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(w.debounce)
			}

		case <-timerCh:
			timer = nil
			timerCh = nil
			if w.onChange != nil {
				w.onChange()
			}

		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}
