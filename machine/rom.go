package machine

import (
	"github.com/peter-mount/go6502/memory"
)

type RomChip struct {
	Filename string `yaml:"filename"`
}

func (c *RomChip) Configure() (memory.Memory, error) {
	rom, err := memory.RomFromFile(c.Filename)
	return rom, err
}
