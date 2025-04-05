package downloader

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/schollz/progressbar/v3"
)

// Metalink XML parsing
type Metalink struct {
	XMLName xml.Name `xml:"metalink"`
	Files   []struct {
		URLs []struct {
			URL string `xml:"url"`
		} `xml:"url"`
	} `xml:"file"`
}

func LogStart(t time.Time, count int) {
	fmt.Printf("%s [NOTICE] Downloading %d item(s)\n", t.Format("01/02 15:04:05"), count)
}

func LogComplete(t time.Time, path string) {
	fmt.Printf("%s [NOTICE] Download complete: %s\n", t.Format("01/02 15:04:05"), path)
}

func PrintResults(gid, status, speed, path string) {
	fmt.Println("Download Results:")
	fmt.Println("gid   |stat|avg speed  |path/URI")
	fmt.Println("======+====+===========+=======================================================")
	fmt.Printf("%s|%-4s|%9s|%s\n", gid, status, speed, path)
	fmt.Println("\nStatus Legend:")
	fmt.Println("(OK):download completed.")
}

func RandHex(n int) string {
	const letters = "0123456789abcdef"
	s := make([]byte, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

// HTTP download with progress bar
func DownloadHTTP(url, filename string) (string, string, error) {
	respHead, err := http.Head(url)
	if err != nil {
		return filename, "", err
	}
	total, err := strconv.Atoi(respHead.Header.Get("Content-Length"))
	if err != nil {
		return filename, "", err
	}
	resp, err := http.Get(url)
	if err != nil {
		return filename, "", err
	}
	defer resp.Body.Close()

	file, err := os.Create(filename)
	if err != nil {
		return filename, "", err
	}
	defer file.Close()

	bar := progressbar.NewOptions(total,
		progressbar.OptionSetDescription("Downloading"),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() { fmt.Fprint(os.Stdout, "\n") }),
	)
	start := time.Now()
	_, err = io.Copy(io.MultiWriter(file, bar), resp.Body)
	if err != nil {
		return filename, "", err
	}
	dur := time.Since(start)
	avg := float64(total) / dur.Seconds()
	return filename, FormatSpeed(avg), nil
}

// Torrent download (magnet or .torrent)
func DownloadTorrent(uri string) (string, string, error) {
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = "./"
	client, err := torrent.NewClient(cfg)
	if err != nil {
		return "", "", err
	}
	defer client.Close()

	var t *torrent.Torrent
	if strings.HasPrefix(uri, "magnet:") {
		t, err = client.AddMagnet(uri)
	} else {
		t, err = client.AddTorrentFromFile(uri)
	}
	if err != nil {
		return "", "", err
	}
	<-t.GotInfo()
	t.DownloadAll()

	total := t.Info().TotalLength()
	bar := progressbar.NewOptions(int(total),
		progressbar.OptionSetDescription("Downloading"),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() { fmt.Fprint(os.Stdout, "\n") }),
	)
	start := time.Now()
	for bar.State().CurrentPercent() < 1.0 {
		bar.Set64(int64(t.Stats().BytesReadData))
		time.Sleep(500 * time.Millisecond)
	}
	dur := time.Since(start)
	avg := float64(total) / dur.Seconds()
	path := t.Files()[0].Path()
	return path, FormatSpeed(avg), nil
}

// Metalink download (uses first URL)
func DownloadMetalink(path string) (string, string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	var ml Metalink
	if err := xml.Unmarshal(data, &ml); err != nil {
		return "", "", err
	}
	if len(ml.Files) == 0 || len(ml.Files[0].URLs) == 0 {
		return "", "", fmt.Errorf("no URLs in metalink")
	}
	url := ml.Files[0].URLs[0].URL
	filename := filepath.Base(url)
	return DownloadHTTP(url, filename)
}

// Format speed in KiB/s or MiB/s
func FormatSpeed(bps float64) string {
	if bps > 1024*1024 {
		return fmt.Sprintf("%6.2fMiB/s", bps/(1024*1024))
	}
	return fmt.Sprintf("%6.2fKiB/s", bps/1024)
}
