package transform

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

func folderList(path string) ([]fileCopy, error) {

	f, err := folderOpen(path)
	if err != nil {
		return []fileCopy{}, err
	}
	defer f.Close()

	list, err := f.Readdir(-1)
	if err != nil {
		return []fileCopy{}, err
	}

	var res = make([]fileCopy, 0, len(list))
	for _, fi := range list {
		if !fi.IsDir() {
			res = append(res, fileCopy{OldName: fi.Name(), Process: allowedExt[filepath.Ext(fi.Name())]})
		}
	}
	return res, nil
}

func listIndexItems(list []fileCopy, hasCover bool) error {
	rep, err := regexp.Compile(`(_preview\.)`)
	if err != nil {
		return err
	}
	//fmt.Println(re.MatchString("surface_0_preview.png"))
	rei, err := regexp.Compile(`^surface_\[(\d+)\]`)
	if err != nil {
		return err
	}
	//m :=re.FindStringSubmatch("surface_[78888](oblozhka)_zone_[0](oblozhka).jpg")

	for i, fi := range list {
		if fi.Process {
			if rep.MatchString(fi.OldName) {
				list[i].Process = false
			} else {
				sm := rei.FindStringSubmatch(fi.OldName)
				if len(sm) != 2 {
					//TODO error?
					list[i].Process = false
				} else {
					idx, err := strconv.Atoi(sm[1])
					if err != nil {
						return err
					}
					if !hasCover {
						idx++
					}
					list[i].SheetIdx = idx
				}
			}
		}
	}
	return nil
}
