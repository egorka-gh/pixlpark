package transform

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var allowedExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".tif":  true,
	".tiff": true,
	".bmp":  true,
	".gif":  true,
}

func replaceRootPath(oldPath, root string) string {
	oldPath = filepath.Clean(oldPath)
	ps := strings.Split(oldPath, string(os.PathSeparator))
	if len(ps) == 0 {
		return ""
	}
	ps[0] = root
	return filepath.Join(ps...)
}

func folderOpen(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fi, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !fi.IsDir() {
		file.Close()
		return nil, fmt.Errorf("Path '%s' is not a folder", path)
	}
	return file, nil
}

func copyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}

	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	//out, err := os.Create(dst)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, sfi.Mode())
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}
