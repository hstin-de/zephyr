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
	MaxStep               = 79
)

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

func (m *IconModel) ProcessParameter(param string, downloadedGribFiles map[string]map[int][]byte, wg *sync.WaitGroup) {
	defer wg.Done()
	var parsedParameter common.ParameterOptions
	var ok bool

	if parsedParameter, ok = common.Parameters[param]; !ok {
		return
	}

	if parsedParameter.StepType == common.ACCUMULATED {

		var previousData []float64 = ndfile.ProcessGRIB(downloadedGribFiles[ParameterLookup[param]][0]).DataValues
		for step := 1; step < MaxStep; step++ {
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

func (m *IconModel) DowloadParameter(parameter []string, fast bool) error {

	GenerateWeights(WeightOptions{
		GridsPath:   "/tmp/gribdl/dwd/grids",
		WeightsPath: "./weights",
		CdoPath:     "cdo",
	})

	var downloadParams []string = make([]string, len(parameter))

	var toDownload []string = make([]string, 0)

	//convert parameter to DWD parameter
	for i, p := range parameter {
		if val, ok := ParameterLookup[p]; ok {
			downloadParams[i] = val
			toDownload = append(toDownload, p)
		}
	}

	Log.Info().Msg("Downloading parameters: " + strings.Join(toDownload, ", "))

	downloadedGribFiles := StartDWDDownloader(DWDOpenDataDownloaderOptions{
		ModelName: m.ModelName,
		Param:     strings.Join(downloadParams, ","),
		MaxStep:   MaxStep,
		Regrid:    true,
		Fast:      fast,
	})

	Log.Info().Msgf("[%s] Download complete. Processing parameters", m.ModelName)

	var wg sync.WaitGroup

	for _, p := range parameter {
		wg.Add(1)
		if fast {
			go m.ProcessParameter(p, downloadedGribFiles, &wg)
		} else {
			m.ProcessParameter(p, downloadedGribFiles, &wg)
		}

	}

	wg.Wait()

	return nil

}
