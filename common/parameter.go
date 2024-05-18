package common

import "time"

var epochTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

type InterpolationMethod int

const (
	LINEAR InterpolationMethod = iota
	COPY
)

type StepType int

const (
	INSTANT StepType = iota
	ACCUMULATED
)

type ParameterOptions struct {
	ParameterID         int
	DisplayName         string
	Unit                string
	InterpolationMethod InterpolationMethod
	StepType            StepType
}

var Parameters map[string]ParameterOptions = map[string]ParameterOptions{
	"temperature":          {ParameterID: 0, DisplayName: "temperature", Unit: "°F", InterpolationMethod: LINEAR, StepType: INSTANT},
	"clouds":               {ParameterID: 67072, DisplayName: "clouds", Unit: "%", InterpolationMethod: LINEAR, StepType: INSTANT},
	"condition":            {ParameterID: 1643264, DisplayName: "condition", Unit: "", InterpolationMethod: COPY, StepType: INSTANT},
	"cape":                 {ParameterID: 395008, DisplayName: "cape", Unit: "J/kg", InterpolationMethod: LINEAR, StepType: INSTANT},
	"wind_u":               {ParameterID: 131584, DisplayName: "wind_u", Unit: "m/s", InterpolationMethod: LINEAR, StepType: INSTANT},
	"wind_v":               {ParameterID: 197120, DisplayName: "wind_v", Unit: "m/s", InterpolationMethod: LINEAR, StepType: INSTANT},
	"relative_humidity":    {ParameterID: 65792, DisplayName: "relative_humidity", Unit: "%", InterpolationMethod: LINEAR, StepType: INSTANT},
	"surface_pressure":     {ParameterID: 768, DisplayName: "surface_pressure", Unit: "Pa", InterpolationMethod: LINEAR, StepType: INSTANT},
	"dewpoint":             {ParameterID: 393216, DisplayName: "dewpoint", Unit: "°F", InterpolationMethod: LINEAR, StepType: INSTANT},
	"snow_depth":           {ParameterID: 721152, DisplayName: "snow_depth", Unit: "m", InterpolationMethod: LINEAR, StepType: INSTANT},
	"surface_pressure_msl": {ParameterID: 66304, DisplayName: "surface_pressure_msl", Unit: "Pa", InterpolationMethod: LINEAR, StepType: INSTANT},
	"precipitation":        {ParameterID: 3408128, DisplayName: "precipitation", Unit: "kg m^-2", InterpolationMethod: LINEAR, StepType: ACCUMULATED},
}
