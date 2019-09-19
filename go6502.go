/*
	go6502 emulates the pda6502 computer. This includes the MOS 6502
	processor, memory-mapping address bus, RAM and ROM, MOS 6522 VIA
	controller, SSD1306 OLED display, and perhaps more.

	Read more at https://github.com/pda/go6502 and https://github.com/pda/pda6502
*/
package main

import (
	"fmt"
	"github.com/peter-mount/go6502/acia6551"
	"os"
	"os/signal"

	"github.com/peter-mount/go6502/bus"
	"github.com/peter-mount/go6502/cli"
	"github.com/peter-mount/go6502/cpu"
	"github.com/peter-mount/go6502/debugger"
	"github.com/peter-mount/go6502/ili9340"
	"github.com/peter-mount/go6502/memory"
	"github.com/peter-mount/go6502/sd"
	"github.com/peter-mount/go6502/speedometer"
	"github.com/peter-mount/go6502/spi"
	"github.com/peter-mount/go6502/ssd1306"
	"github.com/peter-mount/go6502/via6522"
)

const (
	kernalPath = "kernel/kernel.rom"
	//kernalPath  = "rom/kernal.rom"
	//charRomPath = "rom/char.rom"
)

func main() {
	os.Exit(mainReturningStatus())
}

func mainReturningStatus() int {

	options := cli.ParseFlags()

	// Create addressable devices.

	kernal, err := memory.RomFromFile(kernalPath)
	if err != nil {
		panic(err)
	}

	/*
		charRom, err := memory.RomFromFile(charRomPath)
		if err != nil {
			panic(err)
		}
	*/

	ram := &memory.Ram{}

	via := via6522.NewVia6522(via6522.Options{
		DumpAscii:  options.ViaDumpAscii,
		DumpBinary: options.ViaDumpBinary,
	})

	console := acia6551.NewAcia6551(acia6551.Options{
		Peripheral: acia6551.NewConsole(),
	})

	if options.Ili9340 {
		ili9340, err := ili9340.NewDisplay(spi.PinMap{
			Sclk: 0,
			Mosi: 6,
			Miso: 7,
			Ss:   5,
		})
		if err != nil {
			panic(err)
		}
		via.AttachToPortB(ili9340)
	}

	if options.ViaSsd1306 {
		ssd1306 := ssd1306.NewSsd1306()
		via.AttachToPortA(ssd1306)
	}

	if len(options.SdCard) > 0 {
		sd, err := sd.NewSdCardPeripheral(spi.PinMap{
			Sclk: 0,
			Mosi: 6,
			Miso: 7,
			Ss:   4,
		})
		if err != nil {
			panic(err)
		}
		err = sd.LoadFile(options.SdCard)
		if err != nil {
			panic(err)
		}
		via.AttachToPortB(sd)
	}

	via.Reset()

	// Attach devices to address bus.

	addressBus, _ := bus.CreateBus()
	_ = addressBus.Attach(ram, "ram", 0x0000)
	_ = addressBus.Attach(via, "VIA", 0x9000)
	_ = addressBus.Attach(console, "Console", 0x9010)
	//_=addressBus.Attach(charRom, "char", 0xB000)
	_ = addressBus.Attach(kernal, "kernal", 0xF000)

	exitChan := make(chan int, 0)

	cpu := &cpu.Cpu{Bus: addressBus, ExitChan: exitChan}
	defer cpu.Shutdown()
	if options.Debug {
		debugger := debugger.NewDebugger(cpu, options.DebugSymbolFile)
		debugger.QueueCommands(options.DebugCmds)
		cpu.AttachMonitor(debugger)
	} else if options.Speedometer {
		speedo := speedometer.NewSpeedometer()
		cpu.AttachMonitor(speedo)
	}
	cpu.Reset()

	// Dispatch CPU in a goroutine.
	go func() {
		for {
			cpu.Step()
		}
	}()

	var (
		sig        os.Signal
		exitStatus int
	)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	select {
	case exitStatus = <-exitChan:
		// pass
	case sig = <-sigChan:
		fmt.Println("\nGot signal:", sig)
		exitStatus = 1
	}

	fmt.Println(cpu)
	fmt.Println("Dumping RAM into core file")
	ram.Dump("core")

	os.Exit(exitStatus)
	return exitStatus
}
