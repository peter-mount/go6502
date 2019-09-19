package acia6551

import (
	"fmt"
)

type Acia6551 struct {
	rx           byte
	tx           byte
	commandData  byte
	controlData  byte
	rxFull       bool
	txEmpty      bool
	rxIrqEnabled bool
	txIrqEnabled bool
	overrun      bool
	peripheral   SerialPeripheral // The single device that's connected to this serial port
	capabilities int              // capabilities of the peripheral
}

type Options struct {
	Peripheral SerialPeripheral // Peripheral to attach
}

const (
	aciaData = iota
	aciaStatus
	aciaCommand
	aciaControl
)

const (
	Nop           = 0            // Peripheral does nothing
	Read          = 1 << 0       // Capable of reading
	Write         = 1 << 1       // Capable of writing
	BiDirectional = Read | Write // capable of both read & write
)

type SerialPeripheral interface {
	// Returns a bit mask of capabilities, usually one of Read, Write or BiDirectional
	Capabilities() int

	// Read a byte from the device
	// error is non nil if an error occurred
	// bool = true if byte is a value, false if no data was read
	Read() (bool, byte, error)

	// Write a byte to the device
	// bool is true if byte written
	Write(byte) (bool, error)

	// Shutdown the device
	Shutdown()
}

func NewAcia6551(o Options) *Acia6551 {
	acia := &Acia6551{
		peripheral: o.Peripheral,
	}

	// Start backround processes based on the Capabilities
	if acia.peripheral != nil {
		acia.capabilities = acia.peripheral.Capabilities()
	}

	return acia
}

// The address size of the memory-mapped IO.
// Helps to meet the go6502.Memory interface.
func (a *Acia6551) Size() int {
	// We have a only 4 addresses, Data, Status, Command and Control
	return 4
}

func (a *Acia6551) String() string {
	return "ACIA6551"
}

func (a *Acia6551) Shutdown() {
	if a.peripheral != nil {
		a.peripheral.Shutdown()
	}
}

// Emulates a hardware reset
func (a *Acia6551) Reset() {
	a.rx = 0
	a.rxFull = false

	a.tx = 0
	a.txEmpty = true

	a.rxIrqEnabled = false
	a.txIrqEnabled = false

	a.overrun = false

	a.setControl(0)
	a.setCommand(0)
}

func (a *Acia6551) setControl(data byte) {
	a.controlData = data
}

func (a *Acia6551) setCommand(data byte) {
	a.commandData = data

	a.rxIrqEnabled = (data & 0x02) != 0
	a.txIrqEnabled = ((data & 0x04) != 0) && ((data & 0x08) != 1)
}

func (a *Acia6551) statusRegister() byte {
	status := byte(0)

	if a.rxFull {
		status |= 0x08
	}

	if a.txEmpty {
		status |= 0x10
	}

	if a.overrun {
		status |= 0x04
	}

	return status
}

func (a *Acia6551) Read(address uint16) byte {
	switch address {
	default:
		panic(fmt.Sprintf("read from 0x%X not handled by Acia6551", address))
	case aciaData:
		return a.rxRead()
	case aciaStatus:
		return a.statusRegister()
	case aciaCommand:
		return a.commandData
	case aciaControl:
		return a.controlData
	}
}

func (a *Acia6551) Write(address uint16, data byte) {
	switch address {
	case aciaData:
		a.txWrite(data)
	case aciaStatus:
		a.Reset()
	case aciaCommand:
		a.setCommand(data)
	case aciaControl:
		a.setControl(data)
	}
}

func (a *Acia6551) rxRead() byte {
	if !a.rxFull && (a.capabilities&Read) == Read {
		read, data, err := a.peripheral.Read()
		if err != nil {
			// TODO errors
		}
		if read {
			a.rx = data
			// No need to set overrun or rxFull
		}
	}
	a.overrun = false
	a.rxFull = false
	return a.rx
}

func (a *Acia6551) txWrite(data byte) {
	if (a.capabilities & Write) == Write {
		written, err := a.peripheral.Write(data)
		a.tx = data
		a.txEmpty = written || err != nil
	}
}
