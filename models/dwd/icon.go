package dwd

import (
	"hstin/zephyr/common"
	. "hstin/zephyr/helper"
	"os"
	"path"
	"strings"
	"sync"

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
			Log.Info().Msgf("[%s] Processing parameter: %s", m.ModelName, p)
			go common.ProcessParameter(p, downloadedGribFiles, breakPoint, &wg, m.NDFileManager)
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
			Log.Info().Msgf("[%s] Processing parameter: %s", m.ModelName, p)
			common.ProcessParameter(p, downloadedGribFiles, breakPoint, &wg, m.NDFileManager)
		}

	}

	wg.Wait()

	return nil

}
