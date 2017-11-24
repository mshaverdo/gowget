package main

import (
	"fmt"
	"os"
)

func main() {
	urls := os.Args[1:]

	if len(urls) == 0 {
		fmt.Printf("gowget: missing url\nUsage: %s [URL]...\n", os.Args[0])
		os.Exit(1)
	}

	downloader := NewDownloader()
	downloader.Download(urls)
}
