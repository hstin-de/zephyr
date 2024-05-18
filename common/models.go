package common

import (
	"time"
)

func CalculateDaysSinceEpoch(t time.Time) int {
	return int(t.UTC().Sub(epochTime).Hours() / 24)
}
