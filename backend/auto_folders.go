//go:build !cli

package backend

import "github.com/wailsapp/wails/v3/pkg/application"

const autoFolderPickerTitle = "Choose folders for Auto Album"

// ChooseAutoFolders opens the native picker and returns the selected directories.
func (c *ConfigManager) ChooseAutoFolders() ([]string, error) {
	return chooseAutoFolders(application.Get())
}
