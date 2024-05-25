package common

import (
	"fmt"
	"sync"
	"time"

	"github.com/hstin-de/ndfile"
	"golang.org/x/sys/unix"
)

type BaseModel interface {
	GetModelName() string
	GetRootPath() string
	GetParentModel() BaseModel
	DowloadParameter(parameter []string, fast bool) error
}

func CalculateDaysSinceEpoch(t time.Time) int {
	return int(t.UTC().Sub(epochTime).Hours() / 24)
}

func GetFreeMemory() uint64 {
	var info unix.Sysinfo_t
	err := unix.Sysinfo(&info)
	if err != nil {
		fmt.Println("Error getting system info:", err)
		return 0
	}
	return uint64(info.Freeram) * uint64(info.Unit)
}

func ProcessParameter(param string, downloadedGribFiles map[string]map[int][]byte, breakPoint int, wg *sync.WaitGroup, NDFileManager *ndfile.NDFileManager) {
	defer wg.Done()
	var parsedParameter ParameterOptions
	var ok bool

	if parsedParameter, ok = Parameters[param]; !ok {
		return
	}

	newLength := breakPoint + (len(downloadedGribFiles[param])-(breakPoint+1))*3

	var loadedGribFiles map[int][]byte = make(map[int][]byte, newLength)

	for idx, gribFile := range downloadedGribFiles[param] {
		loadedGribFiles[idx] = gribFile
	}

	downloadedGribFiles[param] = nil

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

				if parsedParameter.InterpolationMethod == COPY {
					copy(interpolatedDataValues, prevFile.DataValues)
				} else if parsedParameter.InterpolationMethod == LINEAR {

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

		if parsedParameter.StepType == ACCUMULATED {
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

				NDFileManager.AddGrib(currentGrib)

				currentGrib.DataValues = nil
				currentGrib = ndfile.GRIBFile{}
			}
		} else {
			// Instantaneous data
			NDFileManager.AddGrib(currentGrib)
		}

	}
}
