package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func unzip(archive, target string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		path := filepath.Join(target, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		defer targetFile.Close()

		if _, err := io.Copy(targetFile, fileReader); err != nil {
			return err
		}
	}

	return nil
}

func dirzip(archive, target string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}

	for _, file := range reader.File {
		//path := filepath.Join(target, file.Name)
		path := replaceRootPath(file.Name, target)
		fmt.Printf("%s\n", file.Name)
		fmt.Printf("%s\n", path)
	}

	return nil
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

func main() {
	//dirzip("D:\\Buffer\\tst.zip", "D:\\Buffer\\tst")
	err := unzip("D:\\Buffer\\pp\\wrk\\1850708.zip", "D:\\Buffer\\pp\\wrk\\tst")
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}

}
