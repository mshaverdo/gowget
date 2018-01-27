package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// initialChunkSize is a chunk size in bytes before first speed measurement finished
	initialChunkSize = 10 * 1024
	// chunksPerSecond is an estimated number of chunks, downloaded per one second
	chunksPerSecond = 2
	// filenameSubstitution replace illegal symbols in filename
	filenameSubstitution = "_"
	// defaultFilename is a default filename if filename could not be extracted from URL
	defaultFilename = "index.html"
	// tableUpdateInterval is an interval between percentage table updates
	tableUpdateInterval = 1 * time.Second
)

// printer provides way to intercept output of Downloader when needed
type printer interface {
	// Printf prints standard output
	Printf(format string, a ...interface{}) (n int, err error)
	// ErrPrintf prints erroneous output
	ErrPrintf(format string, a ...interface{}) (n int, err error)
}

// webGetter provides data stream by URL
type webGetter interface {
	// Get returns data body and content length by URL
	Get(url string) (body io.ReadCloser, contentLen int, err error)
}

// Downloader downloads files by specified http urls
type Downloader struct {
	// downloadPercentages stores downloaded percentage for every URL
	downloadPercentages map[string]int
	// Percentages provides sync to prevent concurrent read/write to downloadPercentages map
	percentageMutex sync.RWMutex
	// statusTableRowFormat stores prepared printf format for percentages
	statusTableRowFormat string
	// printer provides output
	printer printer
	// provides data stream by URL
	webGetter webGetter
}

// NewDownloader returns instance of Wget
func NewDownloader() Downloader {
	return Downloader{
		downloadPercentages:  map[string]int{},
		percentageMutex:      sync.RWMutex{},
		statusTableRowFormat: "",
		printer:              stdPrinter{},
		webGetter:            httpWebGetter{},
	}
}

// Download downloads files from specified urls
func (d *Downloader) Download(urls []string) {
	d.downloadPercentages = map[string]int{}
	d.percentageMutex = sync.RWMutex{}

	// remove duplicated urls
	urlsMap := map[string]bool{}
	uniqUrls := []string{}
	for _, url := range urls {
		if !urlsMap[url] {
			uniqUrls = append(uniqUrls, url)
		}
		urlsMap[url] = true
	}

	d.initializeStatusTable(uniqUrls)

	// start downloading
	isFinishedChannels := make([]chan bool, len(uniqUrls))
	for i, url := range uniqUrls {
		isFinishedChannels[i] = make(chan bool, 1)
		go d.downloadUrl(url, isFinishedChannels[i])
	}

	// wait for downloading finishes, print percentage table row every second
	finishedDownloads := 0
	for finishedDownloads < len(uniqUrls) {

		time.Sleep(tableUpdateInterval)
		for _, c := range isFinishedChannels {
			select {
			case <-c:
				finishedDownloads++
			default:
				continue
			}
		}
		d.printStatusTableRow(uniqUrls)
	}
}

// initializeStatusTable calculates format for rows according to filename widths and prints header with filenames
func (d *Downloader) initializeStatusTable(urls []string) {
	filenames := make([]interface{}, len(urls))
	d.statusTableRowFormat = ""
	for i, url := range urls {
		filename := d.getUniqueFilename(d.getFilename(url))
		filenames[i] = filename

		// if filename not longer then 4 chars, use percentage width 3 (3 digits and '%' sign has total width 4)
		// leave width 3 due to percentage cell has width 4
		width := "3"
		if len([]rune(filename)) > 4 {
			// cell width is width modifier + 1 due to '%' sign
			width = strconv.Itoa(len([]rune(filename)) - 1)
		}
		d.statusTableRowFormat += "%" + width + "d%% "
	}

	d.statusTableRowFormat += "\n"

	// use 4 as the minimum width of filename cell
	d.printer.Printf(strings.Repeat("%4s ", len(filenames))+"\n", filenames...)
}

// printStatusTableRow prints percentage row
func (d *Downloader) printStatusTableRow(urls []string) {
	percentages := d.GetDownloadPercentages()
	percentagesSlice := make([]interface{}, len(percentages))
	for i, url := range urls {
		percentagesSlice[i] = percentages[url]
	}
	d.printer.Printf(d.statusTableRowFormat, percentagesSlice...)
}

// Download downloads files from specified urls
func (d *Downloader) downloadUrl(url string, finished chan bool) {
	// initialize percentage
	d.addDownloadPercentage(url, 0)

	filename := d.getUniqueFilename(d.getFilename(url))
	tmpfile, err := ioutil.TempFile("./", filename)
	if err != nil {
		d.printer.ErrPrintf("Unable to create temporary file %s: %s\n", tmpfile, err.Error())
		finished <- false
		return
	}
	defer tmpfile.Close()

	// download
	body, contentLen, err := d.webGetter.Get(url)
	if err != nil {
		d.printer.ErrPrintf("Unable to download URL %s: %s\n", url, err.Error())
		finished <- false
		return
	}
	defer body.Close()

	if contentLen == 0 {
		d.addDownloadPercentage(url, 100)
	}
	chunkLen := initialChunkSize
	totalCopied := 0
	var lastCopied float64
	measurementStart := time.Now()
	for err = nil; err == nil; {
		var n int64
		n, err = io.CopyN(tmpfile, body, int64(chunkLen))
		totalCopied += int(n)
		lastCopied += float64(n)

		if contentLen > 0 {
			d.addDownloadPercentage(url, (totalCopied*100)/contentLen)
		}

		// correct chunkLen according to downloading speed
		if time.Now().Sub(measurementStart) > 1*time.Second {
			// normalize last copied to 1 second
			lastCopiedPerSecond := int(lastCopied / time.Now().Sub(measurementStart).Seconds())
			// update chunk len to copy chunksPerSecond chunks per second
			chunkLen = lastCopiedPerSecond / chunksPerSecond
			lastCopied = 0
			measurementStart = time.Now()
		}
	}

	// rename
	err = os.Rename(tmpfile.Name(), filename)
	if err != nil {
		d.printer.ErrPrintf("Unable to rename %s to %s: %s", tmpfile.Name(), filename, err.Error())
		finished <- false
		return
	}

	// if actually copied bytes equal to contentLen, file downloaded successfully
	finished <- (totalCopied == contentLen)
	return
}

// getUniqueFilename returns unique filename using filename.N schema
func (d *Downloader) getUniqueFilename(filename string) string {
	var err error
	postfix := 1
	uniqueFilename := filename
	for _, err = os.Stat(uniqueFilename); !os.IsNotExist(err); postfix++ {
		uniqueFilename = fmt.Sprintf("%s.%d", filename, postfix)
		_, err = os.Stat(uniqueFilename)
	}
	return uniqueFilename
}

// getFilename returns filename from URL
func (d *Downloader) getFilename(url string) string {
	filename := defaultFilename
	urlRegex := regexp.MustCompile("\\w+://[^/]+(?:/([^/]*))*")
	matches := urlRegex.FindStringSubmatch(url)
	if matches != nil && matches[1] != "" && matches[1] != ".." && matches[1] != "." {
		filename = matches[1]
	}

	re := regexp.MustCompile("[^\\pL\\-.0-9]")
	filename = re.ReplaceAllLiteralString(filename, filenameSubstitution)

	return filename
}

// addDownloadPercentage adds/updates percentage value for URL
func (d *Downloader) addDownloadPercentage(url string, percentage int) {
	d.percentageMutex.Lock()
	defer d.percentageMutex.Unlock()
	d.downloadPercentages[url] = percentage
}

// GetDownloadPercentages returns copy of downloadPercentages map
func (d *Downloader) GetDownloadPercentages() (percentages map[string]int) {
	d.percentageMutex.RLock()
	defer d.percentageMutex.RUnlock()

	percentages = map[string]int{}
	for i, v := range d.downloadPercentages {
		percentages[i] = v
	}

	return percentages
}
