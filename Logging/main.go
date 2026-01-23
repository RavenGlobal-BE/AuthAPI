package logging

import (
	"fmt"
	"log"
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
)

var LogLetterType = map[LogType]string{
	Bold:  "\033[1m",
	Reset: "\033[0m",

	Error:   "\033[31m", //Red
	Fatal:   "\033[35m", //Magenta
	Pass:    "\033[32m", //Green
	Warning: "\033[33m", //Yellow
	Info:    "\033[34m", //Blue
}

func Log(message string, logType LogType) {
	if logType == Fatal {
		log.Fatal(LogLetterType[Fatal] + LogLetterType[Bold] + "ERRO " + LogLetterType[Reset] + message)
	}

	formattedMessage := formatLogMessage(message, logType)
	fmt.Println(formattedMessage)
}
func formatLogMessage(message string, logType LogType) string {
	return fmt.Sprintf("%s%s%s", LogLetterType[logType], message, LogLetterType[Reset])
}
