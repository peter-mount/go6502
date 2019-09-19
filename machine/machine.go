package machine

import (
	"fmt"
	"github.com/peter-mount/go6502/cpu"
	"github.com/peter-mount/go6502/debugger"
	"github.com/peter-mount/go6502/memory"
	"github.com/peter-mount/go6502/speedometer"
	"github.com/peter-mount/golib/kernel"
	"log"
)

type Machine struct {
	config   *Config
	cpu      *cpu.Cpu
	exitChan chan int
}

func (m *Machine) Name() string {
	return "6502"
}

func (m *Machine) Init(k *kernel.Kernel) error {
	svce, err := k.AddService(&Config{})
	if err != nil {
		return err
	}
	m.config = (svce).(*Config)

	return nil
}

func (m *Machine) Start() error {
	m.exitChan = make(chan int, 0)

	m.cpu = &cpu.Cpu{Bus: m.config.addressBus, ExitChan: m.exitChan}

	if m.config.Debug.Debugger {
		debug := debugger.NewDebugger(m.cpu, m.config.Debug.SymbolFile)
		debug.QueueCommands(m.config.Debug.DebugCommands)
		m.cpu.AttachMonitor(debug)
	}

	if m.config.Debug.Speedometer {
		m.cpu.AttachMonitor(speedometer.NewSpeedometer())
	}

	return nil
}

func (m *Machine) Stop() {
	fmt.Println(m.cpu)

	core := m.config.Debug.CoreFile
	if core != "" {
		for id, mem := range m.config.memory {
			if ram, ok := mem.(*memory.Ram); ok {
				filename := fmt.Sprintf("%s-%d.core", core, id)
				fmt.Printf("Dumping ram %d to %s\n", id, filename)
				ram.Dump(filename)
			}
		}
	}
}

func (m *Machine) Run() error {
	m.cpu.Reset()

	running := true

	go func() {
		exitStatus := <-m.exitChan
		log.Println("Exit status", exitStatus)
		running = false
	}()

	for running {
		m.cpu.Step()
	}

	return nil
}
