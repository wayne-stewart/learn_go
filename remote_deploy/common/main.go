package common

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

const DATA_DONE = "DATA_DONE"
const META_BAR = "META|"

const KB float64 = 1024
const MB float64 = KB * KB
const GB float64 = MB * KB
const TB float64 = GB * KB

func FormatBytes(x int) string {
	y := float64(x)
	if y > TB {
		return fmt.Sprintf("%.2f TB", y/TB)
	} else if y > GB {
		return fmt.Sprintf("%.2f GB", y/GB)
	} else if y > MB {
		return fmt.Sprintf("%.2f MB", y/MB)
	} else if y > KB {
		return fmt.Sprintf("%.2f KB", y/KB)
	} else {
		return fmt.Sprintf("%d B", x)
	}
}

func ProgressBytes(count int, total int, message string, pad_right int) {
	fmt.Printf("\r[%.0f%%] %s %-*s", 100.0*float64(count)/float64(total), FormatBytes(count), pad_right, message)
}

func ProgressEach(count int, total int, message string, pad_right int) {
	fmt.Printf("\r%s", ProgressEachValue(count, total, message, pad_right))
}

func ProgressEachValue(count int, total int, message string, pad_right int) string {
	return fmt.Sprintf("[%.0f%%] %d/%d %-*s", 100.0*float64(count)/float64(total), count, total, pad_right, message)
}

type ProgressFunc func(count int, total int, message string, pad_right int)

func EnsureDir(path string) error {
	if _, err := os.Stat(path); err != nil {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

func Compress(src string, dst io.Writer, progress ProgressFunc) (compress_count int, err error) {
	zip_writer := gzip.NewWriter(dst)
	tar_writer := tar.NewWriter(zip_writer)
	total := 0
	count := 0
	pad_right := 0

	fmt.Println("preparing:", src)
	l1 := len("compressing: ")
	filepath.Walk(src, func(file string, info os.FileInfo, err error) error {
		total++
		l := l1 + len(filepath.ToSlash(file[len(src):]))
		if l > pad_right {
			pad_right = l
		}
		return nil
	})

	if err := filepath.Walk(src, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, file)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(file[len(src):])
		if len(header.Name) == 0 {
			header.Name = fmt.Sprintf("ROOT%d", total)
		}
		progress(count, total, "compressing: "+header.Name, pad_right)
		count++

		if err := tar_writer.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tar_writer, data); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		fmt.Println()
		return 0, err
	}
	progress(total, total, "compression complete", pad_right)
	fmt.Println()

	if err := tar_writer.Close(); err != nil {
		return 0, err
	}

	if err := zip_writer.Close(); err != nil {
		return 0, err
	}

	return count, nil
}

func Uncompress(src io.Reader, dst string, progress ProgressFunc) error {
	zip_reader, err := gzip.NewReader(src)
	if err != nil {
		return err
	}

	tar_reader := tar.NewReader(zip_reader)

	header, err := tar_reader.Next()
	if err != nil || header.Name[0:4] != "ROOT" {
		return errors.New("invalid tar.gz format for this appliation, ROOT not found")
	}
	total_items, _ := strconv.Atoi(header.Name[4:])

	err = EnsureDir(dst)
	if err != nil {
		return err
	}

	count := 0

	for {
		header, err := tar_reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dst, header.Name)
		count++
		progress(count, total_items, target, 0)

		switch header.Typeflag {
		case tar.TypeDir:
			err = EnsureDir(target)
			if err != nil {
				return err
			}
		case tar.TypeReg:
			file, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, tar_reader); err != nil {
				file.Close()
				return err
			}
			// defer doesn't work well here in the switch/loop
			// we want files closed immediately
			file.Close()
		}
	}

	return nil
}
