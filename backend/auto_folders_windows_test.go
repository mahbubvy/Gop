//go:build !cli

package backend

import (
	"testing"
	"unsafe"
)

func TestFolderDialogGetResultsVTableSlot(t *testing.T) {
	// IFileOpenDialog inherits slots 0–26 from IFileDialog, ending with SetFilter.
	const getResultsSlot = 27
	var table folderDialogVTable
	if slot := unsafe.Offsetof(table.getResults) / unsafe.Sizeof(uintptr(0)); slot != getResultsSlot {
		t.Fatalf("IFileOpenDialog::GetResults is in slot %d, expected %d", slot, getResultsSlot)
	}
}
