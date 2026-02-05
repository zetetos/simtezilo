// Package hardware initializes and manages hardware components.
package hardware

import (
	"log"

	"periph.io/x/host/v3"
)

// Init initializes the host hardware components.
func Init() {
	_, err := host.Init()
	if err != nil {
		log.Fatal(err)
	}
}
