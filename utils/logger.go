package utils

import (
	"fmt"
	"log"
	"os"
	"time"
	"github.com/mbbgs/sdda-lib/constants"
	"path/filepath"
)

// ANSI color codes
const (
	Red       = "\033[31m"
	Orange    = "\033[38;5;208m"
	NeonGreen = "\033[38;5;46m"
	Reset     = "\033[0m"
)

var logFile *os.File

func InitLogger() error {
	dir, err := GetSessionDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, constants.SDDA_LIB_LOG)
	logFile, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetPrefix("[SDDA_LIB]: ")
	return nil
}

// CloseLogger should be deferred in main()
func CloseLogger() {
	if logFile != nil {
		logFile.Close()
	}
}

func colorLogger(color, label, message string, silent, silentTime bool) {
	now := time.Now().Format("15:04:05")
	var formatted string

	if silentTime {
		formatted = fmt.Sprintf("%s[%s]%s %s\n", color, label, Reset, message)
	} else {
		formatted = fmt.Sprintf("%s[%s %s]%s %s\n", color, label, now, Reset, message)
	}

	if !silent {
		fmt.Print(formatted)
	}

	log.Printf("[%s] %s", label, message)
}


// Quick helpers
func Error(message string) {
	colorLogger(Red, "ERROR", message, false,false)
}

func ErrorE(err error) {
	if err != nil {
		colorLogger(Red, "ERROR", err.Error(),false, false)
	}
}

func WrapError(message string,err error) {
	if err != nil {
		colorLogger(Red, "ERROR",message +"\n"+ err.Error(),false, false)
	}
}



func Warn(message string) {
	colorLogger(Orange, "WARNING", message, false,false)
}

func Done(message string) {
	colorLogger(NeonGreen, "DONE", message, false,false)
}

func SilentDone(message string) {
	colorLogger(NeonGreen, "DONE", message, true,true)
}
