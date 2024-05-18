package main

import (
	. "hstin/zephyr/helper"
	"hstin/zephyr/models/dwd"
	"hstin/zephyr/server"
	"os"
	"sync"

	"github.com/urfave/cli/v2"
)

func main() {

	app := &cli.App{
		Name:  "zephyr - A High-Performance Weather API Server",
		UsageText: "zephyr [global options]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "http",
				Value:   false,
				Usage:   "Start the HTTP server",
				EnvVars: []string{"START_HTTP"},
			},
			&cli.BoolFlag{
				Name:    "grpc",
				Value:   false,
				Usage:   "Start the gRPC server",
				EnvVars: []string{"START_GRPC"},
			},
			&cli.BoolFlag{
				Name:    "download",
				Aliases: []string{"dl"},
				Value:   false,
				Usage:   "Download newest weather data",
				EnvVars: []string{"START_DOWNLOAD"},
			},
			&cli.StringFlag{
				Name:    "http-port",
				Value:   "8081",
				Usage:   "HTTP server port",
				EnvVars: []string{"HTTP_PORT"},
			},
			&cli.StringFlag{
				Name:    "grpc-port",
				Value:   "50051",
				Usage:   "gRPC server port",
				EnvVars: []string{"GRPC_PORT"},
			},
			&cli.StringSliceFlag{
				Name:    "params",
				Aliases: []string{"p"},
				Value:   cli.NewStringSlice("temperature", "clouds", "condition", "cape", "wind_u", "wind_v", "relative_humidity", "surface_pressure", "dewpoint", "snow_depth", "surface_pressure_msl", "precipitation"),
				Usage:   "Parameters to fetch",
				EnvVars: []string{"PARAMS"},
			},
		},
		Action: func(cCtx *cli.Context) error {

			var wg sync.WaitGroup

			if cCtx.Bool("http") {
				wg.Add(1)
				go server.StartServer(cCtx.String("http-port"))
			}

			if cCtx.Bool("grpc") {
				wg.Add(1)
				go server.StartGRPCServer(cCtx.String("grpc-port"))
			}

			if cCtx.Bool("download") {
				model := dwd.NewIconModel(dwd.IconModelOptions{
					RootPath: "data",
				})

				model.DowloadParameter(cCtx.StringSlice("params"))

			}

			wg.Wait()
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		Log.Error().Err(err).Msg("error")
	}
}
