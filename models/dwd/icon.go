package dwd

import (
	"fmt"
	"hstin/zephyr/common"
	. "hstin/zephyr/helper"
	"math"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/hstin-de/ndfile"
	"golang.org/x/sys/unix"
)

const (
	ModelName             = "icon"
	TimeIntervalInMinutes = 60
	MaxStep               = 79
)

var cache map[string]ndfile.NDFile = make(map[string]ndfile.NDFile)

var indexCache map[int64][2]int = make(map[int64][2]int, 0)

var ParameterLookup map[string]string = map[string]string{
	"temperature":          "T_2M",
	"clouds":               "CLCT",
	"condition":            "WW",
	"cape":                 "CAPE_CON",
	"wind_u":               "U_10M",
	"wind_v":               "V_10M",
	"relative_humidity":    "RELHUM_2M",
	"surface_pressure":     "PMSL",
	"dewpoint":             "TD_2M",
	"snow_depth":           "H_SNOW",
	"surface_pressure_msl": "PS",
	"precipitation":        "TOT_PREC",
}

type IconModel struct {
	RootPath      string
	NDFileManager *ndfile.NDFileManager
}

type IconModelOptions struct {
	RootPath string
}

func NewIconModel(opt IconModelOptions) *IconModel {

	rootPath := path.Join(opt.RootPath, ModelName)

	if os.MkdirAll(rootPath, os.ModePerm) != nil {
		Log.Fatal().Msg("Could not create root path for ICON model")
	}

	return &IconModel{
		RootPath:      rootPath,
		NDFileManager: ndfile.NewNDFileManager(rootPath, TimeIntervalInMinutes),
	}
}

func (m *IconModel) GetValues(parameter []common.ParameterOptions, startTime time.Time, forecastDays int, latitude, longitude float64) (map[string][]float64, map[string][]float64, error) {
	daysSinceEpochStart := common.CalculateDaysSinceEpoch(startTime)

	var wg sync.WaitGroup

	// Initialize hourlyData and dailyData maps with initial capacity
	var hourlyData = make(map[string][]float64, len(parameter))
	var dailyData = make(map[string][]float64, len(parameter)*2)

	var hourlyLock sync.Mutex
	var dailyLock sync.Mutex

	// Start concurrent processing for each parameter
	for _, p := range parameter {
		wg.Add(1)
		go func(p common.ParameterOptions) {
			defer wg.Done()

			var steps int

			for day := 0; day <= forecastDays; day++ {
				path := path.Join(m.RootPath, fmt.Sprintf("%d_%d.nd", p.ParameterID, daysSinceEpochStart+day))

				var ndFile ndfile.NDFile
				var err error
				if cachedFile, ok := cache[path]; ok {
					ndFile = cachedFile
				} else {
					ndFile, err = ndfile.PreFetch(path)
					if err != nil {
						continue
					}
					cache[path] = ndFile
				}

				if day == 0 {
					steps = (24 * 60) / int(ndFile.TimeIntervalInMinutes)

					hourlyLock.Lock()
					hourlyData[p.DisplayName] = make([]float64, steps*(forecastDays+1))
					dailyData[p.DisplayName+"_min"] = make([]float64, (forecastDays + 1))
					dailyData[p.DisplayName+"_max"] = make([]float64, (forecastDays + 1))
					hourlyLock.Unlock()
				}

				var latIndex int
				var lngIndex int

				cacheIndex := (int64(latitude/ndFile.Dx) << 32) | (int64(longitude/ndFile.Dy) & 0xFFFFFFFF)

				if cachedIndex, ok := indexCache[cacheIndex]; ok {
					latIndex = cachedIndex[0]
					lngIndex = cachedIndex[1]
				} else {
					latIndex, lngIndex = ndFile.GetIndex(latitude, longitude)
					indexCache[cacheIndex] = [2]int{latIndex, lngIndex}
				}

				values, err := ndFile.GetData(latIndex, lngIndex)
				if err != nil {
					return
				}

				minValue := math.MaxFloat64
				maxValue := -math.MaxFloat64

				startIndex := day * steps

				for j, v := range values {
					if v == 32767 {
						continue
					}

					value := float64(v) / 100.0

					hourlyLock.Lock()
					hourlyData[p.DisplayName][startIndex+j] = value
					hourlyLock.Unlock()

					if value < minValue {
						minValue = value
					}
					if value > maxValue {
						maxValue = value
					}
				}

				dailyLock.Lock()
				dailyData[p.DisplayName+"_min"][day] = minValue
				dailyData[p.DisplayName+"_max"][day] = maxValue
				dailyLock.Unlock()
			}
		}(p)
	}

	wg.Wait()

	return dailyData, hourlyData, nil
}

func getFreeMemory() uint64 {
	var info unix.Sysinfo_t
	err := unix.Sysinfo(&info)
	if err != nil {
		fmt.Println("Error getting system info:", err)
		return 0
	}
	return uint64(info.Freeram) * uint64(info.Unit)
}

func (m *IconModel) ProcessParameter(param string, downloadedGribFiles map[string]map[int][]byte, wg *sync.WaitGroup) {
	defer wg.Done()
	var parsedParameter common.ParameterOptions
	var ok bool

	if parsedParameter, ok = common.Parameters[param]; !ok {
		return
	}

	if parsedParameter.StepType == common.ACCUMULATED {

		var previousData []float64 = ndfile.ProcessGRIB(downloadedGribFiles[ParameterLookup[param]][0]).DataValues
		for step := 0; step < MaxStep; step++ {
			gribFile := ndfile.ProcessGRIB(downloadedGribFiles[ParameterLookup[param]][step])

			origData := gribFile.DataValues

			gribFile.ReferenceTime = gribFile.ReferenceTime.Add(time.Duration(step) * time.Hour)

			for j := 0; j < len(gribFile.DataValues); j++ {
				gribFile.DataValues[j] -= previousData[j]
			}

			copy(origData, previousData)

			m.NDFileManager.AddGrib(gribFile)

			gribFile.DataValues = nil
			gribFile = ndfile.GRIBFile{}
		}

		previousData = nil
	} else {
		for _, gribFile := range downloadedGribFiles[ParameterLookup[param]] {
			m.NDFileManager.AddGrib(ndfile.ProcessGRIB(gribFile))
		}
	}
}

func (m *IconModel) DowloadParameter(parameter []string) error {

	GenerateWeights(WeightOptions{
		GridsPath:   "/tmp/gribdl/dwd/grids",
		WeightsPath: "./weights",
		CdoPath:     "cdo",
	})

	var downloadParams []string = make([]string, len(parameter))

	//convert parameter to DWD parameter
	for i, p := range parameter {
		if val, ok := ParameterLookup[p]; ok {
			downloadParams[i] = val
		}
	}

	downloadedGribFiles := StartDWDDownloader(DWDOpenDataDownloaderOptions{
		ModelName: "icon",
		Param:     strings.Join(downloadParams, ","),
		MaxStep:   MaxStep,
		Regrid:    true,
	})

	ParallelMode := true

	// Each parameter needs 2.5GB of memory
	// Disable parallel processing if not enough memory is available
	if getFreeMemory() < uint64(len(parameter))*uint64(2.5e9) {
		ParallelMode = false
		Log.Warn().Msg("Not enough memory for parallel processing, using compatibility mode! Download will take significantly longer!")
		Log.Warn().Msg("Ensure you have at least 2.5GB of free memory available per parameter!")
	}

	var wg sync.WaitGroup

	for _, p := range parameter {
		wg.Add(1)
		if ParallelMode {
			go m.ProcessParameter(p, downloadedGribFiles, &wg)
		} else {
			m.ProcessParameter(p, downloadedGribFiles, &wg)
		}

	}

	wg.Wait()

	return nil

}
