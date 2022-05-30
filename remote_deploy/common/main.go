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
	"strings"
	"time"
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

// func ProgressBytes(count int, total int, message string, previously_written int) int {
// 	if previously_written > 0 {
// 		fmt.Print(strings.Repeat("\b", previously_written))
// 	}
// 	n, _ := fmt.Printf("[%.0f%%] %s %s", 100.0*float64(count)/float64(total), FormatBytes(count), message)
// 	return n
// }

// func ProgressEach(count int, total int, message string, previously_written int) int {
// 	if previously_written > 0 {
// 		fmt.Print(strings.Repeat("\b", previously_written))
// 	}
// 	n, _ := fmt.Printf("%s", ProgressEachValue(count, total, message))
// 	return n
// }

func ProgressBytesValue(count int, total int, message string) string {
	return fmt.Sprintf("[%.0f%%] %s %s", 100.0*float64(count)/float64(total), FormatBytes(count), message)
}

func ProgressEachValue(count int, total int, message string) string {
	return fmt.Sprintf("[%.0f%%] %d/%d %s", 100.0*float64(count)/float64(total), count, total, message)
}

func ProgressMessageValue(count int, total int, message string) string {
	return message
}

type ProgressFunc func(count int, total int, message string) string

type ProgressInfo struct {
	timeStamp         time.Time
	rateLimit         time.Duration // limit progress updates by rate limit
	previouslyWritten int
	progress          ProgressFunc
	print             bool
}

func (info *ProgressInfo) Write(count int, total int, message string) {
	elapsed := time.Since(info.timeStamp)
	if elapsed > info.rateLimit {
		info.timeStamp = time.Now()
		progress_message := info.progress(count, total, message)
		if info.print {
			message_length := len([]rune(progress_message))
			erase_length := info.previouslyWritten - message_length
			fmt.Print("\r", progress_message)
			if erase_length > 0 {
				fmt.Print(strings.Repeat(" ", erase_length), strings.Repeat("\b", erase_length))
			}
			info.previouslyWritten = message_length
		}
	}
}

func (info *ProgressInfo) Writeln(count int, total int, message string) {
	progress_message := info.progress(count, total, message)
	message_length := len([]rune(progress_message))
	erase_length := info.previouslyWritten - message_length
	if info.print {
		fmt.Print("\r", progress_message)
		if erase_length > 0 {
			fmt.Print(strings.Repeat(" ", erase_length), strings.Repeat("\b", erase_length))
		}
		fmt.Println()
		info.previouslyWritten = 0
	}
}

func (info *ProgressInfo) DisablePrint() {
	info.print = false
}

func BeginProgress(rate_limit time.Duration, progress ProgressFunc) *ProgressInfo {
	var info = new(ProgressInfo)
	info.rateLimit = rate_limit
	info.timeStamp = time.Now()
	info.previouslyWritten = 0
	info.print = true
	info.progress = progress
	return info
}

func EnsureDir(path string) error {
	if _, err := os.Stat(path); err != nil {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

func Compress(src string, dst io.Writer, progress *ProgressInfo) (compress_count int, err error) {
	zip_writer := gzip.NewWriter(dst)
	defer zip_writer.Close()
	tar_writer := tar.NewWriter(zip_writer)
	defer tar_writer.Close()
	total := 0
	count := 0

	// need to walk all files to count them
	filepath.Walk(src, func(file string, info os.FileInfo, err error) error {
		total++
		return nil
	})

	err = filepath.Walk(src, func(file string, info os.FileInfo, err error) error {
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
		progress.Write(count, total, "compressing: "+filepath.Dir(header.Name))
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
	})

	if err != nil {
		fmt.Println()
		return 0, err
	}

	progress.Writeln(total, total, "compression complete")

	// if err := tar_writer.Close(); err != nil {
	// 	return 0, err
	// }

	// if err := zip_writer.Close(); err != nil {
	// 	return 0, err
	// }

	return count, nil
}

func Uncompress(src io.Reader, dst string, progress *ProgressInfo) error {
	zip_reader, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer zip_reader.Close()

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
		progress.Write(count, total_items, filepath.Dir(target))

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

	progress.Writeln(total_items, total_items, "decompression complete")

	return nil
}
