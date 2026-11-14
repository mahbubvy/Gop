//go:build !cli

package backend

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

// These layouts match the COM interfaces from shobjidl_core.h.
type folderDialogVTable struct {
	ole.IUnknownVtbl
	show       uintptr
	_          [5]uintptr
	setOptions uintptr
	getOptions uintptr
	_          [6]uintptr // SetDefaultFolder through GetFileName
	setTitle   uintptr
	_          [9]uintptr // SetOkButtonLabel through SetFilter
	getResults uintptr
}

type folderArrayVTable struct {
	ole.IUnknownVtbl
	_         [4]uintptr
	getCount  uintptr
	getItemAt uintptr
}

type folderItemVTable struct {
	ole.IUnknownVtbl
	_              [2]uintptr
	getDisplayName uintptr
}

func chooseAutoFolders(app *application.App) ([]string, error) {
	return application.InvokeSyncWithResultAndError(func() ([]string, error) {
		var owner uintptr
		if window := app.Window.Current(); window != nil {
			owner = uintptr(window.NativeWindow())
		}
		return showAutoFolderPicker(owner)
	})
}

// shortcut: Wails v3.0.0-beta.12 ignores folder multi-select on Windows (#6263), use its picker once upgraded to a version with the fix.
func showAutoFolderPicker(owner uintptr) ([]string, error) {
	const (
		fosPickFolders      = 0x20
		fosForceFilesystem  = 0x40
		fosAllowMultiselect = 0x200
		sigdnFileSystemPath = 0x80058000
		dialogCancelled     = 0x800704c7
	)
	initErr := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	// S_FALSE also succeeds and must be balanced by CoUninitialize.
	if initErr != nil && initErr.(*ole.OleError).Code() != 1 {
		return nil, fmt.Errorf("initialize folder picker: %w", initErr)
	}
	defer ole.CoUninitialize()

	clsid := ole.NewGUID("{DC1C5A9C-E88A-4DDE-A5A1-60F82A20AEF7}")
	iid := ole.NewGUID("{D57C7288-D4AD-4768-BE02-9D969532D960}")
	dialog, err := ole.CreateInstance(clsid, iid)
	if err != nil {
		return nil, fmt.Errorf("create folder picker: %w", err)
	}
	defer dialog.Release()
	vtable := (*folderDialogVTable)(unsafe.Pointer(dialog.VTable()))

	var options uint32
	hr, _, _ := syscall.SyscallN(vtable.getOptions, uintptr(unsafe.Pointer(dialog)), uintptr(unsafe.Pointer(&options)))
	if err := folderPickerError(hr, "read folder picker options"); err != nil {
		return nil, err
	}
	options |= fosPickFolders | fosForceFilesystem | fosAllowMultiselect
	hr, _, _ = syscall.SyscallN(vtable.setOptions, uintptr(unsafe.Pointer(dialog)), uintptr(options))
	if err := folderPickerError(hr, "enable multi-folder selection"); err != nil {
		return nil, err
	}
	title, err := windows.UTF16PtrFromString(autoFolderPickerTitle)
	if err != nil {
		return nil, err
	}
	hr, _, _ = syscall.SyscallN(vtable.setTitle, uintptr(unsafe.Pointer(dialog)), uintptr(unsafe.Pointer(title)))
	if err := folderPickerError(hr, "set folder picker title"); err != nil {
		return nil, err
	}
	hr, _, _ = syscall.SyscallN(vtable.show, uintptr(unsafe.Pointer(dialog)), owner)
	if uint32(hr) == dialogCancelled {
		return nil, nil
	}
	if err := folderPickerError(hr, "open folder picker"); err != nil {
		return nil, err
	}

	var items *ole.IUnknown
	hr, _, _ = syscall.SyscallN(vtable.getResults, uintptr(unsafe.Pointer(dialog)), uintptr(unsafe.Pointer(&items)))
	if err := folderPickerError(hr, "get selected folders"); err != nil {
		return nil, err
	}
	defer items.Release()
	array := (*folderArrayVTable)(unsafe.Pointer(items.VTable()))
	var count uint32
	hr, _, _ = syscall.SyscallN(array.getCount, uintptr(unsafe.Pointer(items)), uintptr(unsafe.Pointer(&count)))
	if err := folderPickerError(hr, "count selected folders"); err != nil {
		return nil, err
	}

	paths := make([]string, 0, count)
	for index := uint32(0); index < count; index++ {
		var item *ole.IUnknown
		hr, _, _ = syscall.SyscallN(array.getItemAt, uintptr(unsafe.Pointer(items)), uintptr(index), uintptr(unsafe.Pointer(&item)))
		if err := folderPickerError(hr, "get selected folder"); err != nil {
			return nil, err
		}
		var path *uint16
		itemVTable := (*folderItemVTable)(unsafe.Pointer(item.VTable()))
		hr, _, _ = syscall.SyscallN(itemVTable.getDisplayName, uintptr(unsafe.Pointer(item)), sigdnFileSystemPath, uintptr(unsafe.Pointer(&path)))
		item.Release()
		if err := folderPickerError(hr, "read selected folder path"); err != nil {
			return nil, err
		}
		paths = append(paths, windows.UTF16PtrToString(path))
		ole.CoTaskMemFree(uintptr(unsafe.Pointer(path)))
	}
	return paths, nil
}

func folderPickerError(hr uintptr, action string) error {
	if int32(hr) < 0 {
		return fmt.Errorf("%s: %w", action, ole.NewError(uintptr(uint32(hr))))
	}
	return nil
}
