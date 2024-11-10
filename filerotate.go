package FileRotate

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// FileRotate defines a file that will be rotated when it reaches a certain size.
// It will store multiple files, each with a number appended to the end.
// Rotations are done on a best-effort basis, starting when the maximum size is reached. No individual write will be split across multiple files, so the actual file size may be greater than MaxSize depending on when the rotation occurs.
type FileRotate struct {
	options  Options
	curFile  *os.File
	basename string
	pos      int64

	lock sync.Mutex
}

type Options struct {
	MaxCount     int  // Number of files to keep
	MaxSize      int  // Maximum size of each file (in bytes)
	ZStdCompress bool // Compress rotated files with zstd
	MakeDirs     bool // Create directories as needed
}

// New creates a new FileRotate object. The filename is the base name of the file to write to.
func New(filename string, options Options) (*FileRotate, error) {
	if options.MakeDirs {
		dirname := path.Dir(filename)
		if err := os.MkdirAll(dirname, 0700); err != nil {
			return nil, err
		}
	}
	fr := &FileRotate{
		options:  options,
		basename: filename,
	}
	if err := fr.open(); err != nil {
		return nil, err
	}
	return fr, nil
}

func (fr *FileRotate) open() error {
	if fr.curFile != nil {
		return nil
	}
	if stat, err := os.Stat(fr.basename); err == nil {
		fr.pos = stat.Size()
	} else {
		fr.pos = 0
	}
	f, err := os.OpenFile(fr.basename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	//log.Printf("Opened %s at %d", fr.basename, fr.pos)
	fr.curFile = f
	return nil
}

// Close closes the current file.
// If subsequent writes are attempted, the file will be reopened.
func (fr *FileRotate) Close() error {
	fr.lock.Lock()
	defer fr.lock.Unlock()
	f := fr.curFile
	fr.curFile = nil
	if f != nil {
		return f.Close()
	}
	return nil
}

// Write implements the io.Writer interface. It writes data to the current file, and may rotate the file if it exceeds the maximum size.
func (fr *FileRotate) Write(data []byte) (int, error) {
	fr.lock.Lock()
	defer fr.lock.Unlock()
	if fr.curFile == nil {
		if err := fr.open(); err != nil {
			return 0, err
		}
	}
	n, err := fr.curFile.Write(data)
	fr.pos += int64(n)

	go fr.checkRotate()

	return n, err
}

func (fr *FileRotate) checkRotate() {
	fr.lock.Lock()
	defer fr.lock.Unlock()

	if fr.pos < int64(fr.options.MaxSize) {
		return
	}
	//log.Printf("Rotating: %d >= %d", fr.pos, fr.options.MaxSize)
	if err := fr.curFile.Close(); err != nil {
		log.Printf("Couldn't close file %v: %s", fr.curFile, err)
	}
	fr.curFile = nil

	// Move/compress each old file into its new position
	for i := fr.options.MaxCount - 2; i >= 0; i-- {
		filename := fmt.Sprintf("%s.%d", fr.basename, i)
		if fr.options.ZStdCompress {
			filename += ".zst"
		}
		zstdCompress := false
		if i == 0 {
			filename = fr.basename
			zstdCompress = fr.options.ZStdCompress
		}
		if _, err := os.Stat(filename); err == nil {
			if !zstdCompress {
				moveTo := fmt.Sprintf("%s.%d", fr.basename, i+1)
				if fr.options.ZStdCompress {
					moveTo += ".zst"
				}
				//log.Printf("Moving from %s to %s", filename, moveTo)
				if err := os.Rename(filename, moveTo); err != nil {
					log.Printf("Could not rename %s to %s: %s", filename, moveTo, err)
				}
			} else {
				compressTo := fmt.Sprintf("%s.%d.zst", fr.basename, i+1)
				//log.Printf("Compressing from %s to %s", filename, compressTo)
				if c, err := os.OpenFile(compressTo, os.O_CREATE|os.O_WRONLY, 0600); err != nil {
					log.Printf("Unable to open raw compressed output %s: %s", compressTo, err)
				} else if orig, err := os.Open(filename); err != nil {
					log.Printf("Unable to open source %s: %s", filename, err)
				} else if compress, err := zstd.NewWriter(c); err != nil {
					log.Printf("Unable to open compressed output %s: %s", compressTo, err)
				} else if _, err := io.Copy(compress, orig); err != nil {
					log.Printf("Unable to copy compressed output: %s", err)
				} else {
					compress.Close()
					orig.Close()
					c.Close()
					os.Remove(filename)
				}
			}
		}
	}
	if err := fr.open(); err != nil {
		log.Printf("Couldn't open new logfile: %s", err)
		return
	}
}
