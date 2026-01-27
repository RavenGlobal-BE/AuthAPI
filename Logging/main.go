package Logging

import (
	"fmt"
	"os"
	config "raven/auth/Config"
	"time"
)

type LogType int

const (
	Bold LogType = iota
	Reset

	Error
	Fatal
	Pass
	Warning
	Info
	Debug
)

var LogLetterType = map[LogType][2]string{
	Bold:  {"\033[1m"},
	Reset: {"\033[0m"},

	Debug:   {"\033[30m", "DEBG"}, // Gray
	Error:   {"\033[31m", "ERRO"}, // Red
	Fatal:   {"\033[35m", "FATA"}, // Magenta
	Pass:    {"\033[32m", "NOTI"}, // Green
	Warning: {"\033[33m", "WARN"}, // Yellow
	Info:    {"\033[34m", "INFO"}, // Blue
}

func Log(message string, logType LogType) {
	current_time := time.Now()

	if config.AllowVerbose == false && logType == Debug {
		return
	}

	finalizedString := fmt.Sprintf("%s%s%s", current_time.Format("3:04PM "), LogLetterType[logType][0]+LogLetterType[Bold][0]+LogLetterType[logType][1]+" "+LogLetterType[Reset][0], message)
	fmt.Println(finalizedString)

	if logType == Fatal {
		os.Exit(1)
	}

}
