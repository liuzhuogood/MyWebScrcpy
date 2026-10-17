package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"mywebscrcpy/internal/automation"
	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/inputconfig"
	"mywebscrcpy/internal/matchingservice"
	"mywebscrcpy/internal/ocr"
	"mywebscrcpy/internal/serviceprefs"
	"mywebscrcpy/internal/servicepython"
	"mywebscrcpy/internal/template"
	"mywebscrcpy/internal/ws"
)

type auxiliaryServices struct {
	templates   *template.Service
	storage     *template.Storage
	ocr         *ocr.Service
	matching    *matchingservice.Manager
	preferences *serviceprefs.Manager
	automation  *automation.Service
}

func configureAuxiliaryServices(dataDir string, database *db.DB, hub *ws.Hub, mux *http.ServeMux) *auxiliaryServices {
	templateStorage, err := template.NewStorage("")
	if err != nil {
		log.Printf("警告: 初始化模板存储失败: %v，尝试回退到临时目录", err)
		fallbackDir := filepath.Join(os.TempDir(), "mywebscrcpy-templates")
		templateStorage, err = template.NewStorage(fallbackDir)
		if err != nil {
			log.Fatalf("致命错误: 模板存储完全无法初始化: %v", err)
		}
	}
	log.Printf("模板存储目录: %s", templateStorage.BaseDir())
	templateService := template.NewService(templateStorage)
	template.RegisterRoutes(mux, templateService)
	pythonServices := servicepython.New(dataDir)
	var serviceSettings *serviceprefs.Manager
	if database != nil {
		serviceSettings = serviceprefs.New(database)
	} else {
		serviceSettings = serviceprefs.New(nil)
	}
	ocrService := ocr.NewManagedService(database, hub, pythonServices, func(enabled bool, action func() error) error {
		return serviceSettings.Control(servicepython.PaddleOCR, enabled, action)
	})
	ocrService.SetIndraControl(func(enabled bool, action func() error) error {
		return serviceSettings.Control(servicepython.IndraOCR, enabled, action)
	})
	ocrService.RegisterRoutes(mux)
	matchingManager := matchingservice.New(matchingservice.Options{
		Python: func() (string, func(), error) { return pythonServices.Acquire(servicepython.Matching) },
		PreparePython: func(ctx context.Context, stage func(string)) (string, func(), error) {
			return pythonServices.Prepare(ctx, servicepython.Matching, stage)
		},
		Control: func(enabled bool, action func() error) error {
			return serviceSettings.Control(servicepython.Matching, enabled, action)
		},
		Devices: func() []string {
			devices := []string{}
			for _, serial := range hub.MatchingDevices() {
				for _, asset := range templateStorage.ListTemplates(serial) {
					if asset.Enabled {
						devices = append(devices, serial)
						break
					}
				}
			}
			return devices
		},
		TemplateReadable: func(id, serial string) bool {
			asset, err := templateStorage.GetTemplate(id)
			return err == nil && (asset.Serial == serial || asset.Serial == template.GlobalSerial)
		},
		API: mux, Vision: http.HandlerFunc(hub.ServeMatchingVisionWS),
	})
	inputConfig := inputconfig.New(database)
	automationService := automation.NewService(hub, templateStorage, ocrService, inputConfig, dependencies.ManagedADB)
	automationService.RegisterRoutes(mux)

	return &auxiliaryServices{
		templates:   templateService,
		storage:     templateStorage,
		ocr:         ocrService,
		matching:    matchingManager,
		preferences: serviceSettings,
		automation:  automationService,
	}
}

func (s *auxiliaryServices) stopPythonServices() error {
	var failures []error
	for _, service := range []struct {
		name string
		stop func() error
	}{
		{servicepython.Matching, s.matching.Stop},
		{servicepython.PaddleOCR, s.ocr.StopPaddle},
		{servicepython.IndraOCR, s.ocr.StopIndra},
	} {
		if err := s.preferences.StopForRuntimeChange(service.name, service.stop); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", service.name, err))
		}
	}
	return errors.Join(failures...)
}

func (s *auxiliaryServices) restore() {
	s.preferences.RestoreAfter(5*time.Second, map[string]func(context.Context) error{
		servicepython.Matching:  s.matching.Start,
		servicepython.PaddleOCR: s.ocr.StartPaddle,
		servicepython.IndraOCR:  s.ocr.StartIndra,
	})
}

func (s *auxiliaryServices) Close() {
	s.preferences.Close()
	s.matching.Close()
	if err := s.ocr.StopPaddle(); err != nil {
		log.Printf("[OCR] 退出时停止失败: %v", err)
	}
	if err := s.ocr.StopIndra(); err != nil {
		log.Printf("[OCR] 退出时停止 Indra 失败: %v", err)
	}
	s.templates.Stop()
}
