package larder

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"larder/internal/core/model"
	"larder/internal/core/transfer"
)

const maxDataPackageBytes = 64 << 20

func (s *AppService) buildDataPackage(selection transfer.Selection) ([]byte, error) {
	if selection.Empty() {
		return nil, errors.New("select at least one category")
	}
	settings, err := s.core.Settings(s.context())
	if err != nil {
		return nil, err
	}
	data, _, err := transfer.Build(transfer.Options{
		DataDir:      s.dataDir,
		HomeDir:      s.homeDir,
		Projects:     settings.ProjectWorkspaces,
		Settings:     settings,
		Instructions: s.Instructions(),
		Commands:     s.Commands(),
		Skills:       s.core.Snapshot().Skills,
		Selection:    selection,
		AppVersion:   Version,
	})
	return data, err
}

func (s *AppService) ExportDataPackage(selection transfer.Selection) (string, error) {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return "", errors.New("app is not ready")
	}
	data, err := s.buildDataPackage(selection)
	if err != nil {
		return "", err
	}
	dialog := app.Dialog.SaveFile()
	dialog.SetFilename("larder-data-" + time.Now().Format("20060102-150405") + ".zip")
	filterLabel := "Larder data"
	if strings.HasPrefix(strings.ToLower(s.locale), "zh") {
		filterLabel = "Larder 数据"
	}
	dialog.AddFilter(filterLabel, "*.zip")
	if window := app.Window.Current(); window != nil {
		dialog.AttachToWindow(window)
	}
	path, err := dialog.PromptForSingleSelection()
	if err != nil || path == "" {
		return path, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (s *AppService) PickDataPackage() (string, error) {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return "", errors.New("app is not ready")
	}
	title, filterLabel := "Import Larder data", "Larder data"
	if strings.HasPrefix(strings.ToLower(s.locale), "zh") {
		title, filterLabel = "导入 Larder 数据", "Larder 数据"
	}
	dialog := app.Dialog.OpenFile().SetTitle(title)
	dialog.CanChooseFiles(true)
	dialog.CanChooseDirectories(false)
	dialog.AddFilter(filterLabel, "*.zip")
	if window := app.Window.Current(); window != nil {
		dialog.AttachToWindow(window)
	}
	return dialog.PromptForSingleSelection()
}

func (s *AppService) InspectDataPackage(path string) (transfer.Info, error) {
	raw, err := readDataPackage(path)
	if err != nil {
		return transfer.Info{}, err
	}
	return transfer.Inspect(raw)
}

func (s *AppService) ImportDataPackage(path string, selection transfer.Selection) (transfer.Result, error) {
	raw, err := readDataPackage(path)
	if err != nil {
		return transfer.Result{}, err
	}
	if selection.Empty() {
		return transfer.Result{}, errors.New("select at least one category")
	}
	info, err := transfer.Inspect(raw)
	if err != nil {
		return transfer.Result{}, err
	}
	settings, _ := s.core.Settings(s.context())
	projects := append([]string(nil), settings.ProjectWorkspaces...)
	if info.Settings != nil {
		projects = append(projects, info.Settings.ProjectWorkspaces...)
	}
	roots := []string{s.dataDir, s.homeDir}
	roots = append(roots, projects...)
	result, err := transfer.Restore(raw, selection, func(settings *model.Settings) error {
		_, err := s.SaveSettings(*settings)
		return err
	}, roots...)
	if err != nil {
		return transfer.Result{}, err
	}
	return result, nil
}

func readDataPackage(path string) ([]byte, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || clean == "." {
		return nil, errors.New("invalid data package path")
	}
	info, err := os.Stat(clean)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxDataPackageBytes {
		return nil, errors.New("data package is too large")
	}
	return os.ReadFile(clean)
}
