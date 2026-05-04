package secure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ExportPlain writes the supplied bytes to path with private permissions
// (0600 on POSIX, owner-only ACL approximated on Windows by the umask).
// If overwrite is false and the file already exists, ExportPlain returns
// os.ErrExist. The directory is created with mode 0700 if missing.
//
// The supplied data slice is wiped before the function returns.
func ExportPlain(path string, data []byte, overwrite bool) (err error) {
	if path == "" {
		return errors.New("secure: empty export path")
	}
	defer func() {
		for i := range data {
			data[i] = 0
		}
	}()

	dir := filepath.Dir(path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("secure: create export dir: %w", err)
		}
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !overwrite {
		flags |= os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return fmt.Errorf("secure: open export file: %w", err)
	}
	defer func() {
		cerr := f.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("secure: write export file: %w", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(path, 0o600)
	}
	return nil
}
