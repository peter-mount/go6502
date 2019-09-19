package main

import (
	"github.com/peter-mount/go6502/machine"
	"github.com/peter-mount/golib/kernel"
	"log"
)

func main() {
	err := kernel.Launch(&machine.Machine{})
	if err != nil {
		log.Fatal(err)
	}
}
