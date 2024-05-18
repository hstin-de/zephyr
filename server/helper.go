package server

import (
	"errors"
	"hstin/zephyr/common"
	"math"
	"strings"
	"sync"
)

func GetParameterOptions(params string) ([]common.ParameterOptions, error) {
	paramList := strings.FieldsFunc(params, func(c rune) bool { return c == ',' })
	if len(paramList) == 0 {
		return nil, errors.New("no valid parameters specified")
	}

	seenParams := make(map[string]struct{}, len(paramList))
	matchedParams := make([]common.ParameterOptions, 0, len(paramList))

	for _, param := range paramList {
		trimmedParam := strings.TrimSpace(param)
		if _, alreadySeen := seenParams[trimmedParam]; !alreadySeen && trimmedParam != "" {
			if paramOption, ok := common.Parameters[trimmedParam]; ok {
				matchedParams = append(matchedParams, paramOption)
				seenParams[trimmedParam] = struct{}{}
			}
		}
	}

	if len(matchedParams) == 0 {
		return nil, errors.New("no valid parameters specified")
	}

	return matchedParams, nil
}

func calculate15Minutely(hourlyParameter map[string][]float64) map[string][]float64 {

	var wg sync.WaitGroup
	minutely15 := make(map[string][]float64, len(hourlyParameter))
	mu := sync.Mutex{}

	for key, value := range hourlyParameter {
		wg.Add(1)
		go func(key string, value []float64) {
			defer wg.Done()
			n := (len(value) - 1) * 4
			result := make([]float64, n)

			switch common.Parameters[key].InterpolationMethod {
			case common.LINEAR:
				for i := 0; i < len(value)-1; i++ {
					start := value[i]
					diff := (value[i+1] - start) / 4
					baseIndex := i * 4
					result[baseIndex] = start
					result[baseIndex+1] = math.Round((start+diff)*100) / 100
					result[baseIndex+2] = math.Round((start+2*diff)*100) / 100
					result[baseIndex+3] = math.Round((start+3*diff)*100) / 100
				}
				break
			case common.COPY:
				for i := 0; i < len(value)-1; i++ {
					start := value[i]
					baseIndex := i * 4
					result[baseIndex] = start
					result[baseIndex+1] = start
					result[baseIndex+2] = start
					result[baseIndex+3] = start
				}
				break
			}

			mu.Lock()
			minutely15[key] = result
			mu.Unlock()
		}(key, value)
	}

	wg.Wait()
	return minutely15
}
