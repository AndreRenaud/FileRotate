package main

import (
	"flag"
	"log"

	"github.com/AndreRenaud/FileRotate"
)

func main() {
	filename := flag.String("filename", "test.log", "The base name of the file to write to")
	maxCount := flag.Int("maxCount", 10, "Number of files to keep")
	maxSize := flag.Int("maxSize", 1000000, "Maximum size of each file (in bytes)")
	zstdCompress := flag.Bool("zstdCompress", true, "Compress rotated files with zstd")
	makeDirs := flag.Bool("makeDirs", true, "Create directories as needed")
	count := flag.Int("count", 1000000, "Number of log lines to write")
	flag.Parse()

	opt := FileRotate.Options{
		MaxCount:     *maxCount,
		MaxSize:      *maxSize,
		ZStdCompress: *zstdCompress,
		MakeDirs:     *makeDirs,
	}
	fr, err := FileRotate.New(*filename, opt)
	if err != nil {
		panic(err)
	}
	defer fr.Close()

	log.SetOutput(fr)

	for i := 0; i < *count; i++ {
		log.Printf("Log entry %d\n", i)
	}
}
