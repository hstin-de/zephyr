package helper

import (
	"os"

	"github.com/phuslu/log"
)

var Log log.Logger = log.Logger{
	Writer: &log.ConsoleWriter{
		Writer:      os.Stdout,
		ColorOutput: true,
	},
}
