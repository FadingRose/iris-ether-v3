package db

import (
	"os"
	"path/filepath"
	"strings"
)

func listDirectories(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var directories []string
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, filepath.Join(path, entry.Name()))
		}
	}
	return directories, nil
}

func listFiles(path string, exts ...string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for _, ext := range exts {
			if strings.HasSuffix(entry.Name(), ext) {
				files = append(files, entry.Name())
			}
		}
	}
	return files, nil
}

func findRuntimeBinFiles(path string) (string, error) {
	var file string
	var candidates []string
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".bin-runtime") {
			candidates = append(candidates, p)
		}
		return nil
	})

	var largestSize int64
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.Size() > largestSize {
			file = candidate
		}
	}
	if err != nil {
		return "", err
	}
	return file, nil
}
