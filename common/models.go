package common

import (
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

func CalculateDaysSinceEpoch(t time.Time) int {
	return int(t.UTC().Sub(epochTime).Hours() / 24)
}

func GetFreeMemory() uint64 {
	var info unix.Sysinfo_t
	err := unix.Sysinfo(&info)
	if err != nil {
		fmt.Println("Error getting system info:", err)
		return 0
	}
	return uint64(info.Freeram) * uint64(info.Unit)
}
