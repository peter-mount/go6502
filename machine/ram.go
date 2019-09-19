package machine

import (
	"fmt"
	"github.com/peter-mount/go6502/memory"
)

type RamChip struct {
	Size uint16 `yaml:"size"`
}

func (c *RamChip) Configure() (memory.Memory, error) {
	// Min 1K chip
	if c.Size < 1024 {
		return nil, fmt.Errorf("Invalid ram size %d", c.Size)
	}

	// FIXME make ram size configurable
	return &memory.Ram{}, nil
}
