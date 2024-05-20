package dwd

import (
	"compress/bzip2"
	"embed"
	. "hstin/zephyr/helper"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed icon_weights/*
var weights embed.FS

type WeightOptions struct {
	GridsPath   string
	WeightsPath string
	CdoPath     string
}

type WeightsDetails struct {
	CDOOptions      string
	DescriptionFile string
	SampleFile      string
	GridFile        string
}

var weightFiles map[string]WeightsDetails = map[string]WeightsDetails{
	"icon-d2_weights.nc": {
		CDOOptions:      ":2",
		DescriptionFile: "icon-d2_description.txt",
		SampleFile:      "icon-d2_sample.grib2",
		GridFile:        "icon_grid_0047_R19B07_L.nc",
	},
	"icon-d2-eps_weights.nc": {
		CDOOptions:      ":2",
		DescriptionFile: "icon-d2-eps_description.txt",
		SampleFile:      "icon-d2-eps_sample.grib2",
		GridFile:        "icon_grid_0047_R19B07_L.nc",
	},
	"icon-eu-eps_weights.nc": {
		CDOOptions:      ":1",
		DescriptionFile: "icon-eu-eps_description.txt",
		SampleFile:      "icon-eu-eps_sample.grib2",
		GridFile:        "icon_grid_0028_R02B07_N02.nc",
	},
	"icon-eps_weights.nc": {
		CDOOptions:      ":1",
		DescriptionFile: "icon-eps_description.txt",
		SampleFile:      "icon-eps_sample.grib2",
		GridFile:        "icon_grid_0024_R02B06_G.nc",
	},
	"icon_weights.nc": {
		CDOOptions:      ":1",
		DescriptionFile: "icon_description.txt",
		SampleFile:      "icon_sample.grib2",
		GridFile:        "icon_grid_0026_R03B07_G.nc",
	},
}

func GenerateWeights(opts WeightOptions) {
	os.MkdirAll(opts.GridsPath, os.ModePerm)
	os.MkdirAll(opts.WeightsPath, os.ModePerm)

	// copy the description and sample files to the weights path
	files, err := weights.ReadDir("icon_weights")
	if err != nil {
		Log.Fatal().Err(err).Msg("Error reading weights directory")
	}

	for _, file := range files {
		if _, err := os.Stat(filepath.Join(opts.WeightsPath, file.Name())); err != nil {
			src, err := weights.Open("icon_weights/" + file.Name())
			if err != nil {
				Log.Fatal().Err(err).Msg("Error opening file")
			}
			defer src.Close()

			dest, err := os.Create(filepath.Join(opts.WeightsPath, file.Name()))
			if err != nil {
				Log.Fatal().Err(err).Msg("Error creating file")
			}
			defer dest.Close()

			io.Copy(dest, src)

		}
	}

	var wg sync.WaitGroup

	for weightFile, details := range weightFiles {
		wg.Add(1)
		go func(weightFile string, details WeightsDetails) {
			defer wg.Done()
			if _, err := os.Stat(filepath.Join(opts.WeightsPath, weightFile)); err != nil {
				Log.Info().Msg("Need to generate weights for " + weightFile)

				dest := filepath.Join(opts.GridsPath, details.GridFile)

				resp, err := http.Get("https://opendata.dwd.de/weather/lib/cdo/" + details.GridFile + ".bz2")
				if err != nil {
					Log.Error().Err(err).Msg("Error downloading grid file")
					return
				}
				defer resp.Body.Close()
				bz2Reader := bzip2.NewReader(resp.Body)
				outFile, err := os.Create(dest)
				if err != nil {
					Log.Error().Err(err).Msg("Error creating grid file")
					return
				}
				defer outFile.Close()
				io.Copy(outFile, bz2Reader)

				args := []string{
					"gennn," + filepath.Join(opts.WeightsPath, details.DescriptionFile),
					"-setgrid," + filepath.Join(opts.GridsPath, details.GridFile) + details.CDOOptions,
					filepath.Join(opts.WeightsPath, details.SampleFile),
					filepath.Join(opts.WeightsPath, weightFile),
				}

				cmd := exec.Command(opts.CdoPath, args...)
				if err := cmd.Run(); err != nil {
					Log.Fatal().Err(err).Msg("Error generating weights")
				}

			}
		}(weightFile, details)
	}

	wg.Wait()

	Log.Info().Msg("Weights loaded successfully")
}
