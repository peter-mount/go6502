package machine

import (
	"github.com/peter-mount/go6502/acia6551"
	"github.com/peter-mount/go6502/memory"
)

type Acia6551Chip struct {
	Peripheral string `yaml:"peripheral"`
}

func (c *Acia6551Chip) Configure() (memory.Memory, error) {
	var peripheral acia6551.SerialPeripheral

	if c.Peripheral == "console" {
		peripheral = acia6551.NewConsole()
	}

	return acia6551.NewAcia6551(acia6551.Options{
		Peripheral: peripheral,
	}), nil
}
