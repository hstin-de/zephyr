package dwd

import (
	"hstin/zephyr/common"
	. "hstin/zephyr/helper"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/hstin-de/ndfile"
)

const (
	TimeIntervalInMinutes = 60
	MaxStep               = 180
)

type IconModel struct {
	RootPath      string
	ModelName     string
	NDFileManager *ndfile.NDFileManager
	ParentModel   common.BaseModel
}

type IconModelOptions struct {
	RootPath    string
	ModelName   string
	ParentModel common.BaseModel
}

func NewIconModel(opt IconModelOptions) *IconModel {

	rootPath := path.Join(opt.RootPath, opt.ModelName)

	if os.MkdirAll(rootPath, os.ModePerm) != nil {
		Log.Fatal().Msgf("Could not create root path for model '%s'", opt.ModelName)
	}

	return &IconModel{
		RootPath:      rootPath,
		ModelName:     opt.ModelName,
		NDFileManager: ndfile.NewNDFileManager(rootPath, TimeIntervalInMinutes),
		ParentModel:   opt.ParentModel,
	}
}

func (m *IconModel) GetRootPath() string {
	return m.RootPath
}

func (m *IconModel) GetModelName() string {
	return m.ModelName
}

func (m *IconModel) GetParentModel() common.BaseModel {
	return m.ParentModel
}

var gribFileMutex sync.Mutex

func (m *IconModel) ProcessParameter(param string, downloadedGribFiles map[string]map[int][]byte, breakPoint int, wg *sync.WaitGroup) {
	defer wg.Done()
	var parsedParameter common.ParameterOptions
	var ok bool

	if parsedParameter, ok = common.Parameters[param]; !ok {
		return
	}

	newLength := breakPoint + (len(downloadedGribFiles[param])-(breakPoint+1))*3

	var loadedGribFiles map[int][]byte = make(map[int][]byte, newLength)

	gribFileMutex.Lock()
	for idx, gribFile := range downloadedGribFiles[param] {
		loadedGribFiles[idx] = gribFile
	}

	downloadedGribFiles[param] = nil
	gribFileMutex.Unlock()

	Log.Info().Msgf("[%s] Processing parameter: %s", m.ModelName, param)

	var previousData []float64

	for step := 0; step <= newLength; step++ {

		var currentGrib ndfile.GRIBFile

		if _, ok := loadedGribFiles[step]; !ok {
			// Hour not available, interpolate
			prevIndex := (step / 3) * 3
			nextIndex := prevIndex + 3

			if nextIndex <= newLength {
				prevFile := ndfile.ProcessGRIB(loadedGribFiles[prevIndex])
				nextFile := ndfile.ProcessGRIB(loadedGribFiles[nextIndex])

				prevTime := prevFile.ReferenceTime
				nextTime := nextFile.ReferenceTime

				// Calculate the interpolated time
				interpolatedTime := prevTime.Add(time.Duration(step-prevIndex) * (nextTime.Sub(prevTime) / 3))

				// Interpolate DataValues
				interpolatedDataValues := make([]float64, len(prevFile.DataValues))

				if parsedParameter.InterpolationMethod == common.COPY {
					copy(interpolatedDataValues, prevFile.DataValues)
				} else if parsedParameter.InterpolationMethod == common.LINEAR {

					for j := range interpolatedDataValues {
						prevValue := prevFile.DataValues[j]
						nextValue := nextFile.DataValues[j]
						interpolatedDataValues[j] = prevValue + float64(step-prevIndex)*(nextValue-prevValue)/3
					}
				}

				currentGrib = prevFile

				copy(currentGrib.DataValues, interpolatedDataValues)
				currentGrib.ReferenceTime = interpolatedTime
			}
		} else {
			// Hour available
			currentGrib = ndfile.ProcessGRIB(loadedGribFiles[step])
		}

		if parsedParameter.StepType == common.ACCUMULATED {
			// Accumulated data, subtract previous data

			// setup previous data
			if step == 0 {
				previousData = currentGrib.DataValues
			} else {

				origData := currentGrib.DataValues

				currentGrib.ReferenceTime = currentGrib.ReferenceTime.Add(time.Duration(step) * time.Hour)

				for j := 0; j < len(currentGrib.DataValues); j++ {
					currentGrib.DataValues[j] -= previousData[j]
				}

				copy(previousData, origData)

				m.NDFileManager.AddGrib(currentGrib)

				currentGrib.DataValues = nil
				currentGrib = ndfile.GRIBFile{}
			}
		} else {
			// Instantaneous data
			m.NDFileManager.AddGrib(currentGrib)
		}

	}
}

func (m *IconModel) DowloadParameter(parameter []string, fast bool) error {

	GenerateWeights(WeightOptions{
		GridsPath:   "/tmp/gribdl/dwd/grids",
		WeightsPath: "./weights",
		CdoPath:     "cdo",
	})

	var downloadParams []string = make([]string, len(parameter))

	//convert parameter to DWD parameter
	for i, p := range parameter {
		if val, ok := common.Parameters[p]; ok {
			downloadParams[i] = val.DisplayName
		}
	}

	Log.Info().Msg("Downloading parameters: " + strings.Join(downloadParams, ", "))

	var wg sync.WaitGroup

	if fast {

		downloadedGribFiles, breakPoint := StartDWDDownloader(DWDOpenDataDownloaderOptions{
			ModelName: m.ModelName,
			Params:    downloadParams,
			MaxStep:   MaxStep,
			Regrid:    true,
			Fast:      fast,
		})

		Log.Info().Msgf("[%s] Download complete. Processing parameters", m.ModelName)

		for _, p := range downloadParams {
			wg.Add(1)
			go m.ProcessParameter(p, downloadedGribFiles, breakPoint, &wg)
		}

	} else {

		for _, p := range downloadParams {
			downloadedGribFiles, breakPoint := StartDWDDownloader(DWDOpenDataDownloaderOptions{
				ModelName: m.ModelName,
				Params:    []string{p},
				MaxStep:   MaxStep,
				Regrid:    true,
			})

			wg.Add(1)
			m.ProcessParameter(p, downloadedGribFiles, breakPoint, &wg)
		}

	}

	wg.Wait()

	return nil

}
