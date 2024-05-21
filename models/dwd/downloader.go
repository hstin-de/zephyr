package dwd

import (
	"compress/bzip2"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	. "hstin/zephyr/helper"

	_ "golang.org/x/net/http2"
)

type DWDModel struct {
	model                         string
	openDataDeliveryOffsetMinutes int
	intervalHours                 int
	grid                          string
	area                          string
	urlFormat                     string
	maxStep                       map[int]int
	breakPoint                    int
}

var dwdModels = map[string]DWDModel{
	"icon": {
		model:                         "icon",
		openDataDeliveryOffsetMinutes: 240,
		intervalHours:                 6,
		grid:                          "icosahedral",
		area:                          "global",
		urlFormat:                     "https://opendata.dwd.de/weather/nwp/%sL/grib/%s/%sL/%sL_%s_%s_single-level_%s%s_%s_%sU.grib2.bz2",
		maxStep: map[int]int{
			0:  180,
			6:  120,
			12: 180,
			18: 120,
		},
		breakPoint: 78,
	},
	"icon-d2": {
		model:                         "icon-d2",
		openDataDeliveryOffsetMinutes: 540,
		intervalHours:                 12,
		grid:                          "icosahedral",
		area:                          "germany",
		urlFormat:                     "https://opendata.dwd.de/weather/nwp/%sL/grib/%s/%sL/%sL_%s_%s_single-level_%s%s_%s_2d_%sL.grib2.bz2",
		maxStep: map[int]int{
			0:  180,
			6:  120,
			12: 180,
			18: 120,
		},
		breakPoint: 24,
	},
	"icon-eu": {
		model:                         "icon-eu",
		openDataDeliveryOffsetMinutes: 240,
		intervalHours:                 3,
		grid:                          "regular-lat-lon",
		area:                          "europe",
		urlFormat:                     "https://opendata.dwd.de/weather/nwp/%sL/grib/%s/%sL/%sL_%s_%s_single-level_%s%s_%s_%sU.grib2.bz2",
		maxStep: map[int]int{
			0:  120,
			3:  30,
			6:  120,
			9:  30,
			12: 120,
			15: 30,
			18: 120,
			21: 30,
		},
		breakPoint: 78,
	},
}

type DWDOpenDataDownloader struct {
	modelName       string
	param           string
	tmpFolder       string
	descriptionFile string
	weightsFile     string
	maxStep         int
	regrid          bool
	modelDetails    DWDModel
	httpClient      *http.Client
	Fast            bool
}

type DWDOpenDataDownloaderOptions struct {
	ModelName    string
	Param        string
	OutputFolder string
	MaxStep      int
	Regrid       bool
	ModelDetails DWDModel
	Fast         bool
}

func formatString(format string, args ...interface{}) string {
	formatParts := strings.Split(format, "%")
	var result string

	for i, part := range formatParts {
		if i == 0 && !strings.HasPrefix(part, "sU") && !strings.HasPrefix(part, "sL") && !strings.HasPrefix(part, "s") {
			result += part
			continue
		}

		if len(part) == 0 {
			continue
		}

		switch {
		case strings.HasPrefix(part, "sU"):
			if i-1 < len(args) {
				result += strings.ToUpper(fmt.Sprint(args[i-1]))
			}
			result += part[2:]
		case strings.HasPrefix(part, "sL"):
			if i-1 < len(args) {
				result += strings.ToLower(fmt.Sprint(args[i-1]))
			}
			result += part[2:]
		case strings.HasPrefix(part, "s"):
			if i-1 < len(args) {
				result += fmt.Sprint(args[i-1])
			}
			result += part[1:]
		default:
			result += "%" + part
		}
	}

	return result
}

func NewDWDOpenDataDownloader(options DWDOpenDataDownloaderOptions) *DWDOpenDataDownloader {

	tmpFolder := "/tmp/gribdl/dwd"

	currentPath, err := os.Getwd()
	if err != nil {
		Log.Error().Err(err).Msg("Error getting current path")
	}
	weightsDir := currentPath + "/weights"

	if _, err := os.Stat(tmpFolder); os.IsNotExist(err) {
		os.MkdirAll(tmpFolder, 0755)
	}

	return &DWDOpenDataDownloader{
		modelName:       options.ModelName,
		param:           options.Param,
		tmpFolder:       tmpFolder,
		descriptionFile: weightsDir + "/" + dwdModels[options.ModelName].model + "_description.txt",
		weightsFile:     weightsDir + "/" + dwdModels[options.ModelName].model + "_weights.nc",
		maxStep:         options.MaxStep,
		regrid:          options.Regrid,
		modelDetails:    options.ModelDetails,
		httpClient:      &http.Client{Timeout: 5 * time.Minute},
		Fast:            options.Fast,
	}
}

func (wdp *DWDOpenDataDownloader) getMostRecentModelTimestamp() time.Time {
	offset := time.Duration(-wdp.modelDetails.openDataDeliveryOffsetMinutes) * time.Minute
	return time.Now().UTC().Add(offset).Truncate(time.Duration(wdp.modelDetails.intervalHours) * time.Hour)
}

func (wdp *DWDOpenDataDownloader) getGribFileUrl(param string, date time.Time, step int) string {
	hour := fmt.Sprintf("%02d", date.UTC().Hour())
	year, month, day := date.UTC().Date()
	model := wdp.modelDetails.model

	return formatString(wdp.modelDetails.urlFormat,
		model, hour,
		param, model,
		wdp.modelDetails.area, wdp.modelDetails.grid,
		fmt.Sprintf("%04d%02d%02d", year, month, day), hour, fmt.Sprintf("%03d", step),
		param)
}

func (wdp *DWDOpenDataDownloader) regridFile(filePath string) string {

	regridFile := strings.Replace(filePath, ".grib2", "_regrid.grib2", -1)

	cmd := exec.Command("cdo", "-f", "grb2", "remap,"+wdp.descriptionFile+","+wdp.weightsFile, filePath, regridFile)
	if err := cmd.Run(); err != nil {
		Log.Error().Err(err).Msg("Error regridding file")
		return filePath
	}

	if err := os.Remove(filePath); err != nil {
		Log.Error().Err(err).Msg("Error removing original file")
		return filePath
	}

	return regridFile

}

func (wdp *DWDOpenDataDownloader) downloadAndProcessFile(url string, retries int) ([]byte, error) {
	resp, err := wdp.httpClient.Get(url)
	if err != nil {
		if retries > 0 {
			Log.Info().Msgf("[DL] Retrying... Error: %s", err)
			return wdp.downloadAndProcessFile(url, retries-1)
		}
		return nil, fmt.Errorf("[DL] getting url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if retries > 0 {
			Log.Info().Msgf("[DL] Retrying.... Status code: %d", resp.StatusCode)
			return wdp.downloadAndProcessFile(url, retries-1)
		}
		return nil, fmt.Errorf("[DL] non-200 status code: %d", resp.StatusCode)
	}

	bz2Reader := bzip2.NewReader(resp.Body)
	fileName := filepath.Base(url)

	filePath := filepath.Join(wdp.tmpFolder, strings.TrimSuffix(fileName, ".bz2"))

	outputFile, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("[DL] creating file: %w", err)
	}
	defer outputFile.Close()

	if _, err = io.Copy(outputFile, bz2Reader); err != nil {
		if retries > 0 {
			Log.Info().Msgf("[DL] Retrying.... Error: %s", err)
			return wdp.downloadAndProcessFile(url, retries-1)
		}
		return nil, fmt.Errorf("[DL] copying file: %w", err)
	}

	if wdp.regrid && wdp.modelDetails.grid != "regular-lat-lon" {
		filePath = wdp.regridFile(filePath)
	}

	gribFile, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("[MOVE] opening source file: %w", err)
	}

	err = os.Remove(filePath)
	if err != nil {
		return nil, fmt.Errorf("[MOVE] removing original file: %w", err)
	}

	return gribFile, nil
}

func (wdp *DWDOpenDataDownloader) DownloadStep(param string, step int, timestamp time.Time) ([]byte, error) {
	url := wdp.getGribFileUrl(param, timestamp, step)
	gribFile, err := wdp.downloadAndProcessFile(url, 5)
	if err != nil {
		return nil, err
	}

	return gribFile, nil
}

func StartDWDDownloader(options DWDOpenDataDownloaderOptions) map[string]map[int][]byte {
	modelDetails, exists := dwdModels[options.ModelName]
	if !exists {
		Log.Error().Msg("Model not found")
		for key := range dwdModels {
			Log.Info().Msg(key)
		}
		return nil
	}

	options.ModelDetails = modelDetails

	wdp := NewDWDOpenDataDownloader(options)

	timestamp := wdp.getMostRecentModelTimestamp()

	if wdp.maxStep > options.ModelDetails.maxStep[timestamp.Hour()] {
		wdp.maxStep = options.ModelDetails.maxStep[timestamp.Hour()]
	}

	params := strings.Split(wdp.param, ",")

	gribFiles := make(map[string]map[int][]byte)
	var mu sync.Mutex
	var wg sync.WaitGroup

	downloadStep := func(p string, step int) ([]byte, error) {
		gribFile, err := wdp.DownloadStep(p, step, timestamp)
		if err != nil {
			return nil, err
		}
		return gribFile, nil
	}

	processParam := func(p string) {
		defer wg.Done()
		firstLoop := wdp.maxStep

		if wdp.maxStep >= wdp.modelDetails.breakPoint {
			firstLoop = wdp.modelDetails.breakPoint
		}

		for step := 0; step < firstLoop; step++ {
			gribFile, err := downloadStep(p, step)
			if err != nil {
				return
			}
			mu.Lock()
			gribFiles[p][step] = gribFile
			mu.Unlock()
			Log.Info().Msgf("Downloaded %s %d/%d", p, step, wdp.maxStep-1)
		}

		for step := wdp.modelDetails.breakPoint; step <= wdp.maxStep; step += 3 {
			gribFile, err := downloadStep(p, step)
			if err != nil {
				return
			}
			mu.Lock()
			gribFiles[p][step] = gribFile
			mu.Unlock()
			Log.Info().Msgf("Downloaded %s %d/%d", p, step, wdp.maxStep-1)
		}
	}

	Log.Info().Msgf("Downloading %s with Fast Mode: %t", wdp.modelName, wdp.Fast)

	for _, p := range params {
		mu.Lock()
		gribFiles[p] = make(map[int][]byte)
		mu.Unlock()

		if wdp.Fast {
			wg.Add(1)
			go processParam(p)
		} else {
			wg.Add(1)
			processParam(p)
		}
	}

	wg.Wait()

	wg.Wait()

	return gribFiles
}
