//go:build !windows && !cli

package backend

import "github.com/wailsapp/wails/v3/pkg/application"

func chooseAutoFolders(app *application.App) ([]string, error) {
	return app.Dialog.OpenFile().AttachToWindow(app.Window.Current()).
		SetTitle(autoFolderPickerTitle).SetButtonText("Choose").
		CanChooseDirectories(true).CanChooseFiles(false).PromptForMultipleSelection()
}
