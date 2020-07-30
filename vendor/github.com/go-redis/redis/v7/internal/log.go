package internal

import (
	"log"
	"os"
)

var Logger = log.New(os.Stderr, "redis: ", log.LstdFlags|log.Lshortfile)
