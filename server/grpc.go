package server

import (
	"context"
	"errors"
	"hstin/zephyr/common"
	"hstin/zephyr/models/base"
	"hstin/zephyr/protobuf"
	"net"
	"os"
	"time"

	. "hstin/zephyr/helper"

	"github.com/zsefvlol/timezonemapper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/structpb"
)

const grpcEnvVar = "ZEPHYR_GRPC_SERVER_RUNNING"

type server struct {
	protobuf.UnimplementedForecastServiceServer
}

func (s *server) GetForecast(ctx context.Context, in *protobuf.ForecastRequest) (*protobuf.ForecastResponse, error) {
	startCalculation := time.Now()

	if in.Lat < -90 || in.Lat > 90 {
		return nil, errors.New("invalid latitude")
	}

	if in.Lng < -180 || in.Lng > 180 {
		return nil, errors.New("invalid longitude")
	}

	var params []common.ParameterOptions = make([]common.ParameterOptions, 0, len(in.Parameters))

	for _, param := range in.Parameters {
		if paramOption, ok := common.Parameters[param]; ok {
			params = append(params, paramOption)
		}
	}

	model, _ := base.GetBestModel(in.Lat, in.Lng, in.Model)

	timezone := timezonemapper.LatLngToTimezoneString(in.Lat, in.Lng)

	matchedParams, err := GetParameterOptions(in.Parameters)
	if err != nil {
		return nil, err
	}

	forecastDays := int(in.ForecastDays)
	if forecastDays > 365 {
		return nil, errors.New("Invalid number of days")
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}

	startTime := time.Now().In(loc)

	_, offset := startTime.Zone()

	dailyParameter, hourlyParameter, usedModels, err := base.GetValues(model, matchedParams, startTime, forecastDays, in.Lat, in.Lng)
	if err != nil {
		return nil, errors.New("Error getting data")
	}

	var daily map[string]*structpb.ListValue = make(map[string]*structpb.ListValue, len(dailyParameter))
	var hourly map[string]*structpb.ListValue = make(map[string]*structpb.ListValue, len(hourlyParameter))
	var minutely15 map[string]*structpb.ListValue = make(map[string]*structpb.ListValue, len(hourlyParameter))
	var usedModelsMap map[string]*structpb.ListValue = make(map[string]*structpb.ListValue, len(usedModels))

	for key, value := range dailyParameter {
		daily[key] = &structpb.ListValue{
			Values: make([]*structpb.Value, len(value)),
		}

		for i, val := range value {
			daily[key].Values[i] = &structpb.Value{
				Kind: &structpb.Value_NumberValue{
					NumberValue: val,
				},
			}
		}
	}

	for key, value := range hourlyParameter {
		hourly[key] = &structpb.ListValue{
			Values: make([]*structpb.Value, len(value)),
		}

		for i, val := range value {
			hourly[key].Values[i] = &structpb.Value{
				Kind: &structpb.Value_NumberValue{
					NumberValue: val,
				},
			}
		}
	}

	if in.Minutely15 {
		minutely15Tmp := calculate15Minutely(hourlyParameter)
		for key, value := range minutely15Tmp {
			minutely15[key] = &structpb.ListValue{
				Values: make([]*structpb.Value, len(value)),
			}

			for i, val := range value {
				minutely15[key].Values[i] = &structpb.Value{
					Kind: &structpb.Value_NumberValue{
						NumberValue: val,
					},
				}
			}
		}
	}

	for key, value := range usedModels {
		usedModelsMap[key] = &structpb.ListValue{
			Values: make([]*structpb.Value, len(value)),
		}

		for i, val := range value {
			usedModelsMap[key].Values[i] = &structpb.Value{
				Kind: &structpb.Value_StringValue{
					StringValue: val,
				},
			}
		}
	}

	return &protobuf.ForecastResponse{
		CalculationTime: int64(time.Since(startCalculation).Microseconds()),
		Latitude:        in.Lat,
		Longitude:       in.Lng,
		UtcOffset:       int32(offset * 1000),
		Timezone:        timezone,
		StartTime:       startTime.Truncate(24*time.Hour).Unix() * 1000,
		UsedModels:      usedModelsMap,
		Daily:           daily,
		Hourly:          hourly,
		Minutely15:      minutely15,
	}, nil

}

func StartGRPCServer(port string) {

	// Only start the gRPC server once
	if os.Getenv(grpcEnvVar) != "" {
		return
	}
	os.Setenv(grpcEnvVar, "true")

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		Log.Fatal().Err(err).Msg("failed to start listener")
	}

	s := grpc.NewServer()
	protobuf.RegisterForecastServiceServer(s, &server{})
	reflection.Register(s)
	Log.Info().Msgf("gRPC server listening at :%s", port)
	if err := s.Serve(lis); err != nil {
		Log.Fatal().Err(err).Msg("failed to start gRPC server")
	}
}
