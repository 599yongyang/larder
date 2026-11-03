package platform

import "os"

// Linker is the write surface used only after a Plan is explicitly confirmed.
// It deliberately exposes only symlink creation and removal; replacements and
// arbitrary file deletion need a separate, reviewed operation.
type Linker interface {
	Symlink(source, target string) error
	Remove(path string) error
}

type OSLinker struct{}

// Remove deletes a link without touching whatever it points at. A real
// directory is refused: only links may be removed.
func (OSLinker) Remove(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return os.ErrInvalid
	}
	return os.Remove(path)
}
