package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kzn/downloader"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	flag.Usage = func() {
		fmt.Println("Usage: kzn <URL|magnet|.torrent|.metalink> [output]")
		os.Exit(1)
	}
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
	}
	uri := args[0]
	var output string
	if len(args) >= 2 {
		output = args[1]
	}

	// Log start
	downloader.LogStart(time.Now(), 1)
	gid := downloader.RandHex(6)
	status := "OK"

	// Dispatch by URI type
	path, speed, err := func() (string, string, error) {
		switch {
		case strings.HasPrefix(uri, "magnet:"), strings.HasSuffix(strings.ToLower(uri), ".torrent"):
			return downloader.DownloadTorrent(uri)
		case strings.HasSuffix(strings.ToLower(uri), ".metalink"):
			return downloader.DownloadMetalink(uri)
		default:
			if output == "" {
				output = filepath.Base(uri)
			}
			return downloader.DownloadHTTP(uri, output)
		}
	}()
	if err != nil {
		status = "ERR"
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	// Log complete and results
	downloader.LogComplete(time.Now(), path)
	fmt.Println()
	downloader.PrintResults(gid, status, speed, path)
}
