package noaa

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
	MaxStep               = 384
)

type GFSModel struct {
	RootPath      string
	ModelName     string
	NDFileManager *ndfile.NDFileManager
	ParentModel   common.BaseModel
}

type GFSModelOptions struct {
	RootPath    string
	ModelName   string
	ParentModel common.BaseModel
}

func NewGFSModel(opt GFSModelOptions) *GFSModel {

	rootPath := path.Join(opt.RootPath, opt.ModelName)

	if os.MkdirAll(rootPath, os.ModePerm) != nil {
		Log.Fatal().Msgf("Could not create root path for model '%s'", opt.ModelName)
	}

	return &GFSModel{
		RootPath:      rootPath,
		ModelName:     opt.ModelName,
		NDFileManager: ndfile.NewNDFileManager(rootPath, TimeIntervalInMinutes),
		ParentModel:   opt.ParentModel,
	}
}

func (m *GFSModel) GetRootPath() string {
	return m.RootPath
}

func (m *GFSModel) GetModelName() string {
	return m.ModelName
}

func (m *GFSModel) GetParentModel() common.BaseModel {
	return m.ParentModel
}

func (m *GFSModel) DowloadParameter(parameter []string, fast bool) error {

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

		_, _ = StartNOAADownloader(NOAADownloaderOptions{
			ModelName: m.ModelName,
			Params:    downloadParams,
			MaxStep:   MaxStep,
			Height:    "surface",
			Fast:      fast,
		})

		Log.Info().Msgf("[%s] Download complete. Processing parameters", m.ModelName)

		// for _, p := range downloadParams {
		// 	wg.Add(1)
		// 	go m.ProcessParameter(p, downloadedGribFiles, breakPoint, &wg)
		// }

	} else {

		for _, p := range downloadParams {
			downloadedGribFiles, breakPoint := StartNOAADownloader(NOAADownloaderOptions{
				ModelName: m.ModelName,
				Params:    []string{p},
				MaxStep:   MaxStep,
				Height:    "surface",
				Fast:      fast,
			})

			wg.Add(1)
			common.ProcessParameter(p, downloadedGribFiles, breakPoint, &wg, m.NDFileManager)
		}

	}

	wg.Wait()

	return nil

}
