package dwd

import (
	"fmt"
	"hstin/zephyr/common"
	"math"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/hstin-de/ndfile"
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

func (m *IconModel) DowloadParameter(parameter []string) error {

	GenerateWeights(WeightOptions{
		GridsPath:   "/tmp/gribdl/dwd/grids",
		WeightsPath: "/home/coding/hstin/zephyr/weights",
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

	var wg sync.WaitGroup

	for _, p := range parameter {
		wg.Add(1)
		go func(param string) {
			defer wg.Done()
			var parsedParameter common.ParameterOptions
			var ok bool

			if parsedParameter, ok = common.Parameters[param]; !ok {
				return
			}

			gribFiles := make(map[int]ndfile.GRIBFile)

			for step := 0; step < MaxStep; step++ {
				parsedFile := ndfile.ProcessGRIB(downloadedGribFiles[ParameterLookup[param]][step])
				gribFiles[step] = parsedFile
			}

			if parsedParameter.StepType == common.ACCUMULATED {

				var previousData []float64 = gribFiles[0].DataValues
				for i := 1; i < len(gribFiles); i++ {

					gribFile := gribFiles[i]

					currentData := gribFile.DataValues

					gribFile.DataValues = make([]float64, len(currentData))

					gribFile.ReferenceTime = gribFile.ReferenceTime.Add(time.Duration(i) * time.Hour)

					for i := 0; i < len(currentData); i++ {
						gribFile.DataValues[i] = currentData[i] - previousData[i]
					}

					previousData = currentData

					m.NDFileManager.AddGrib(gribFile)

				}
			} else {
				for _, gribFile := range gribFiles {
					m.NDFileManager.AddGrib(gribFile)
				}
			}

		}(p)

	}

	wg.Wait()

	return nil

}
