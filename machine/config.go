package machine

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"github.com/peter-mount/go6502/bus"
	"github.com/peter-mount/go6502/memory"
	"github.com/peter-mount/golib/kernel"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"path/filepath"
)

type Config struct {
	Debug struct {
		Debugger      bool     `yaml:"debugger"`
		DebugCommands []string `yaml:"debugCommands"`
		SymbolFile    string   `yaml:"symbolFile"`
		Speedometer   bool     `yaml:"speedometer"`
		CoreFile      string   `yaml:"dumpCore"`
	} `yaml:"debug"`
	Hardware   []Hardware `yaml:"hardware"`
	configFile *string
	addressBus *bus.Bus
	memory     []memory.Memory
}

type Hardware struct {
	Name     string        `yaml:"name"`
	Address  string        `yaml:"address"`
	Ram      *RamChip      `yaml:"ram"`
	Rom      *RomChip      `yaml:"rom"`
	Acia6551 *Acia6551Chip `yaml:"6551"`
	Via6522  *Via6522Chip  `yaml:"6522"`
}

type Chip interface {
	Configure() (memory.Memory, error)
}

func (c *Config) Name() string {
	return "config"
}

func (c *Config) Init(k *kernel.Kernel) error {
	c.configFile = flag.String("c", "", "The config file to use")

	return nil
}

func (c *Config) PostInit() error {
	// Verify then load the config file
	if *c.configFile == "" {
		*c.configFile = "config.yaml"
	}

	if filename, err := filepath.Abs(*c.configFile); err != nil {
		return err
	} else if in, err := ioutil.ReadFile(filename); err != nil {
		return err
	} else if err := yaml.Unmarshal(in, c); err != nil {
		return err
	}

	return nil
}

func (c *Config) Start() error {
	addressBus, err := bus.CreateBus()
	if err != nil {
		return err
	}

	c.addressBus = addressBus

	for _, h := range c.Hardware {
		var address uint16

		if h.Address == "" {
			return fmt.Errorf("Invalid Hardware entry, name %s", h.Name)
		}

		b, err := hex.DecodeString(h.Address)
		if err != nil {
			return err
		}
		if len(b) != 2 {
			return fmt.Errorf("Invalid Address, name %s, got %s", h.Name, h.Address)
		}
		address = (uint16(b[0]) << 8) | uint16(b[1])

		err = errors.New("No chip defined")
		if h.Ram != nil {
			err = c.attach(h.Name, address, h.Ram)
		} else if h.Rom != nil {
			err = c.attach(h.Name, address, h.Rom)
		} else if h.Acia6551 != nil {
			err = c.attach(h.Name, address, h.Acia6551)
		} else if h.Via6522 != nil {
			err = c.attach(h.Name, address, h.Via6522)
		}
		if err != nil {
			return err
		}
	}

	return nil
}
func (c *Config) attach(name string, address uint16, chip Chip) error {
	m, err := chip.Configure()
	if err != nil {
		return err
	}

	err = c.addressBus.Attach(m, name, address)
	if err != nil {
		return err
	}

	c.memory = append(c.memory, m)
	return nil
}
