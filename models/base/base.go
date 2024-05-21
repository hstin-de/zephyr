package base

import (
	"hstin/zephyr/common"
	"hstin/zephyr/models/dwd"
	"time"
)

type BaseModel interface {
	GetValues(parameter []common.ParameterOptions, startTime time.Time, forecastDays int, latitude, longitude float64) (map[string][]float64, map[string][]float64, error)
	DowloadParameter(parameter []string, fast bool) error
}

var iconModel = dwd.NewIconModel(dwd.IconModelOptions{
	RootPath: "data",
})

func GetBestModel(latitude, longitude float64) BaseModel {
	return iconModel
}
