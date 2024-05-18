package server

import (
	"context"
	"errors"
	"hstin/zephyr/common"
	"hstin/zephyr/models/base"
	"hstin/zephyr/protobuf"
	"net"
	"time"

	. "hstin/zephyr/helper"

	"github.com/zsefvlol/timezonemapper"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

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

	model := base.GetBestModel(in.Lat, in.Lng)

	timezone := timezonemapper.LatLngToTimezoneString(in.Lat, in.Lng)

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}

	startTime := time.Now().In(loc)

	_, offset := startTime.Zone()

	dailyParameter, hourlyParameter, err := model.GetValues(params, startTime, int(in.ForecastDays), in.Lat, in.Lng)
	if err != nil {
		return nil, err
	}

	var daily map[string]*structpb.ListValue = make(map[string]*structpb.ListValue, len(dailyParameter))
	var hourly map[string]*structpb.ListValue = make(map[string]*structpb.ListValue, len(hourlyParameter))
	// var minutely15 map[string]*structpb.ListValue = make(map[string]*structpb.ListValue, 0)

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

	return &protobuf.ForecastResponse{
		CalculationTime: time.Since(startCalculation).Microseconds(),
		Latitude:        in.Lat,
		Longitude:       in.Lng,
		UtcOffset:       int32(offset * 1000),
		Timezone:        timezone,
		StartTime:       startTime.Truncate(24*time.Hour).Unix() * 1000,
		Daily:           daily,
		Hourly:          hourly,
	}, nil

}

func StartGRPCServer(port string) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		Log.Fatal().Err(err).Msg("failed to start listener")
	}

	s := grpc.NewServer()
	protobuf.RegisterForecastServiceServer(s, &server{})
	Log.Printf("gRPC server listening at :%s", port)
	if err := s.Serve(lis); err != nil {
		Log.Fatal().Err(err).Msg("failed to start gRPC server")
	}
}
