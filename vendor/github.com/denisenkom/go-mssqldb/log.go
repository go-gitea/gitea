package mssql

import (
	"log"
)

type Logger log.Logger

func (logger *Logger) Printf(format string, v ...interface{}) {
	if logger != nil {
		(*log.Logger)(logger).Printf(format, v...)
	} else {
		log.Printf(format, v...)
	}
}

func (logger *Logger) Println(v ...interface{}) {
	if logger != nil {
		(*log.Logger)(logger).Println(v...)
	} else {
		log.Println(v...)
	}
}
