package machine

import (
	"github.com/peter-mount/go6502/memory"
	"github.com/peter-mount/go6502/via6522"
)

type Via6522Chip struct {
	DumpAscii  bool `yaml:"dumpAscii"`
	DumpBinary bool `yaml:"dumpBinary"`
}

func (c *Via6522Chip) Configure() (memory.Memory, error) {
	return via6522.NewVia6522(via6522.Options{
		DumpAscii:  c.DumpAscii,
		DumpBinary: c.DumpBinary,
	}), nil
}
