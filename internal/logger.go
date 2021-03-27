package internal

import (
	"fmt"
	"log"
	"os"
)

const (
	LogOptions = log.LstdFlags | log.Lmsgprefix | log.Lshortfile
)

func FunctionLogger(prefix string) *log.Logger {
	return log.New(os.Stdout, fmt.Sprintf("%s] ", prefix), LogOptions)
}
