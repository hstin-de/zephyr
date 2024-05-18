package server

import (
	. "hstin/zephyr/helper"
	"hstin/zephyr/models/base"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xhhuango/json"
	"github.com/zsefvlol/timezonemapper"
)

type ForecastResponse struct {
	CalculationTime int64                `json:"calculation_time"`
	Latitude        float64              `json:"latitude"`
	Longitude       float64              `json:"longitude"`
	UTCOffset       int                  `json:"utc_offset"`
	Timezone        string               `json:"timezone"`
	StartTime       int64                `json:"start_time"`
	Daily           map[string][]float64 `json:"daily"`
	Hourly          map[string][]float64 `json:"hourly"`
	Minitely15      map[string][]float64 `json:"minutely15"`
}

func StartServer(port string) {

	app := fiber.New(fiber.Config{
		JSONEncoder:           json.Marshal,
		JSONDecoder:           json.Unmarshal,
		Prefork:               true,
		Concurrency:           256 * 1024 * 1024 * 24,
		DisableStartupMessage: true,
		ServerHeader:          "zephyr",
	})

	app.Get("/forecast", func(c *fiber.Ctx) error {
		startCalculation := time.Now()
		latitude := c.QueryFloat("lat")
		if latitude < -90 || latitude > 90 { // Valid latitude range
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid latitude"})
		}

		longitude := c.QueryFloat("lng")
		if longitude < -180 || longitude > 180 { // Valid longitude range
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid longitude"})
		}

		timezone := timezonemapper.LatLngToTimezoneString(latitude, longitude)
		params := c.Query("params")

		if params == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No parameters specified"})
		}

		matchedParams, err := GetParameterOptions(params)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		forecastDays := c.QueryInt("forecastDays")
		if forecastDays > 365 { // Reasonable number of forecast days
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid number of days"})
		}

		loc, err := time.LoadLocation(timezone)
		if err != nil {
			loc = time.UTC
		}

		startTime := time.Now().In(loc)

		_, offset := startTime.Zone()

		model := base.GetBestModel(latitude, longitude)

		dailyParameter, hourlyParameter, err := model.GetValues(matchedParams, startTime, forecastDays, latitude, longitude)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Error getting data"})
		}

		var minutely15 map[string][]float64 = make(map[string][]float64, 0)

		if c.QueryBool("minutely15") {
			minutely15 = calculate15Minutely(hourlyParameter)
		}

		return c.JSON(ForecastResponse{
			CalculationTime: time.Since(startCalculation).Microseconds(),
			Latitude:        latitude,
			Longitude:       longitude,
			UTCOffset:       offset * 1000,
			Timezone:        timezone,
			StartTime:       startTime.Truncate(24*time.Hour).Unix() * 1000,
			Daily:           dailyParameter,
			Hourly:          hourlyParameter,
			Minitely15:      minutely15,
		})
	})

	Log.Info().Msg("HTTP server started on port " + port)

	Log.Fatal().Err(app.Listen(":" + port)).Msg("Failed to start HTTP server")
}
