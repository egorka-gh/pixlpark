package transform

import (
	"os"
	"path/filepath"
	"strings"
)

func replaceRootPath(oldPath, root string) string {
	oldPath = filepath.Clean(oldPath)
	ps := strings.Split(oldPath, string(os.PathSeparator))
	if len(ps) == 0 {
		return ""
	}
	ps[0] = root
	return filepath.Join(ps...)
}
