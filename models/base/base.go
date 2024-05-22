package base

import (
	"fmt"
	"hstin/zephyr/common"

	// . "hstin/zephyr/helper"
	"hstin/zephyr/models/dwd"
	"math"
	"path"
	"sync"
	"time"

	"github.com/hstin-de/ndfile"
)

var cache map[string]ndfile.NDFile = make(map[string]ndfile.NDFile)

var indexCache map[string]map[int64][2]int = make(map[string]map[int64][2]int, 0)

// Worldwide ICON model
var iconModel = dwd.NewIconModel(dwd.IconModelOptions{
	RootPath:  "data",
	ModelName: "icon",
})

// ICON-EU model
var iconEUModel = dwd.NewIconModel(dwd.IconModelOptions{
	RootPath:    "data",
	ModelName:   "icon-eu",
	ParentModel: iconModel,
})

// ICON-D2 model
var iconD2Model = dwd.NewIconModel(dwd.IconModelOptions{
	RootPath:    "data",
	ModelName:   "icon-d2",
	ParentModel: iconEUModel,
})

type Border struct {
	LatMax float64
	LatMin float64
	LngMax float64
	LngMin float64
}

type ModelOptions struct {
	Border Border
	Model  common.BaseModel
}

var AvailableModels = map[string]ModelOptions{
	"icon":    {Model: iconModel, Border: Border{LatMax: 90, LatMin: -90, LngMax: 180, LngMin: -180}},
	"icon-eu": {Model: iconEUModel, Border: Border{LatMax: 70.5, LatMin: 29.5, LngMax: 62.5, LngMin: -23.5}},
	"icon-d2": {Model: iconD2Model, Border: Border{LatMax: 70.5, LatMin: 29.5, LngMax: 62.5, LngMin: -23.5}},
}

func GetBestModel(latitude, longitude float64, preferredModel string) (common.BaseModel, string) {

	// Check if a preferred model is given and available within the model map and given coordinates
	if preferredModel != "" && preferredModel != "auto" {
		if matchedModelOptions, ok := AvailableModels[preferredModel]; ok {
			choosenModel := matchedModelOptions.Model
			choosenModelBorder := matchedModelOptions.Border

			for {
				if latitude >= choosenModelBorder.LatMin && latitude <= choosenModelBorder.LatMax && longitude >= choosenModelBorder.LngMin && longitude <= choosenModelBorder.LngMax {
					return choosenModel, preferredModel
				}

				parentModel := choosenModel.GetParentModel()
				if parentModel == nil {
					break
				}

				preferredModel = parentModel.GetModelName()
				if parentModelOptions, exists := AvailableModels[preferredModel]; exists {
					choosenModel = parentModel
					choosenModelBorder = parentModelOptions.Border
				} else {
					break // If the parent model is not in the map, exit the loop
				}
			}
		}
	}

	// No preferred model or no matching preferred model, use the best model for the given coordinates

	// ICON-EU
	// Coordinates from: https://dwd-geoportal.de/products/G_D5M/
	if latitude >= 29.5 && latitude <= 70.5 && longitude >= -23.5 && longitude <= 62.5 {
		return iconEUModel, "icon-eu"
	}

	// Default to the ICON model
	return iconModel, "icon"
}

func GetNDFile(model common.BaseModel, parameterID, daysSinceEpoch int) (ndfile.NDFile, common.BaseModel, error) {
	path := path.Join(model.GetRootPath(), fmt.Sprintf("%d_%d.nd", parameterID, daysSinceEpoch))

	if cachedFile, ok := cache[path]; ok {
		return cachedFile, model, nil
	}

	ndFile, err := ndfile.PreFetch(path)
	if err != nil {

		//check if the model has a parent model. If so, try to get the file from the parent model, if not just continue
		if model.GetParentModel() == nil {
			return ndfile.NDFile{}, nil, err
		}

		return GetNDFile(model.GetParentModel(), parameterID, daysSinceEpoch)
	}

	cache[path] = ndFile

	return ndFile, model, nil
}

func GetValues(model common.BaseModel, parameter []common.ParameterOptions, startTime time.Time, forecastDays int, latitude, longitude float64) (map[string][]float64, map[string][]float64, map[string][]string, error) {
	daysSinceEpochStart := common.CalculateDaysSinceEpoch(startTime)

	var wg sync.WaitGroup

	// Initialize hourlyData and dailyData maps with initial capacity
	var hourlyData = make(map[string][]float64, len(parameter))
	var dailyData = make(map[string][]float64, len(parameter)*2)

	var usedModels = make(map[string]map[string]bool, 0)

	modelName := model.GetModelName()

	if _, ok := indexCache[modelName]; !ok {
		indexCache[modelName] = make(map[int64][2]int, 0)
	}

	var hourlyLock sync.Mutex
	var dailyLock sync.Mutex
	var usedModelsLock sync.Mutex

	// Start concurrent processing for each parameter
	for _, p := range parameter {
		wg.Add(1)
		go func(p common.ParameterOptions) {
			defer wg.Done()

			var steps int

			for day := 0; day <= forecastDays; day++ {

				ndFile, model, err := GetNDFile(model, p.ParameterID, daysSinceEpochStart+day)
				if err != nil {
					continue
				}

				usedModelsLock.Lock()
				if _, ok := usedModels[p.DisplayName]; !ok {
					usedModels[p.DisplayName] = make(map[string]bool, 0)
				}

				usedModels[p.DisplayName][model.GetModelName()] = true
				usedModelsLock.Unlock()

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

				if cachedIndex, ok := indexCache[modelName][cacheIndex]; ok {
					latIndex = cachedIndex[0]
					lngIndex = cachedIndex[1]
				} else {
					latIndex, lngIndex = ndFile.GetIndex(latitude, longitude)
					indexCache[modelName][cacheIndex] = [2]int{latIndex, lngIndex}
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

	// "param" : ["model1", model2...]
	var usedModelsMap = make(map[string][]string, 0)

	for param, models := range usedModels {
		for model := range models {
			usedModelsMap[param] = append(usedModelsMap[param], model)
		}
	}

	return dailyData, hourlyData, usedModelsMap, nil
}
