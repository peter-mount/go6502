/*
	Package debugger provides an interactive stepping debugger for go6502 with
	breakpoints on instruction type, register values and memory location.

	Example

	An example interactive debugging session:

		$ go run go6502.go --via-ssd1306 --debug
		CPU PC:0xF31F AC:0x00 X:0x00 Y:0x00 SP:0x00 SR:--_b-i--
		Next: SEI implied
		$F31F> step
		CPU PC:0xF320 AC:0x00 X:0x00 Y:0x00 SP:0x00 SR:--_b----
		Next: LDX immediate $FF
		$F320> break-register X $FF
		Breakpoint set: X = $FF (255)
		$F320> continue
		Breakpoint for X = $FF (255)
		CPU PC:0xF322 AC:0x00 X:0xFF Y:0x00 SP:0x00 SR:n-_b----
		Next: TXS implied
		$F322> step
		Breakpoint for X = $FF (255)
		CPU PC:0xF323 AC:0x00 X:0xFF Y:0x00 SP:0xFF SR:n-_b----
		Next: CLI implied
		$F323>
		Breakpoint for X = $FF (255)
		CPU PC:0xF324 AC:0x00 X:0xFF Y:0x00 SP:0xFF SR:n-_b-i--
		Next: CLD implied
		$F324>
		Breakpoint for X = $FF (255)
		CPU PC:0xF325 AC:0x00 X:0xFF Y:0x00 SP:0xFF SR:n-_b-i--
		Next: JMP absolute $F07B
		$F325> break-instruction nop
		$F325> r
		Breakpoint for X = $FF (255)
		CPU PC:0xF07B AC:0x00 X:0xFF Y:0x00 SP:0xFF SR:n-_b-i--
		Next: LDA immediate $00
		$F07B> q
*/
package debugger

/**
 * TODO:
 * -  Command argument validation.
 * -  Handle missing/multiple labels when entering address.
 * -  Resolve addresses to symbols non-absolute instructions, e.g. branch.
 * -  Tab completion from commands, not just debug symbols.
 * -  `step n` e.g. `step 100` to step 100 instructions.
 * -  Read and write CLI history file.
 */

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/peter-mount/go6502/cpu"
	"github.com/peterh/liner"
)

const (
	debugCmdNone = iota
	debugCmdBreakAddress
	debugCmdBreakInstruction
	debugCmdBreakRegister
	debugCmdContinue
	debugCmdExit
	debugCmdHelp
	debugCmdInvalid
	debugCmdNext
	debugCmdRead
	debugCmdRead16
	debugCmdRead32
	debugCmdStep
)

type Debugger struct {
	symbols           debugSymbols
	inputQueue        []string
	cpu               *cpu.Cpu
	liner             *liner.State
	lastCmd           *cmd
	run               bool
	breakAddress      bool
	breakAddressValue uint16
	breakInstruction  string
	breakRegA         bool
	breakRegAValue    byte
	breakRegX         bool
	breakRegXValue    byte
	breakRegY         bool
	breakRegYValue    byte
}

type cmd struct {
	id        int
	input     string
	arguments []string
}

// NewDebugger creates a debugger.
// Be sure to defer a call to Debugger.Shutdown() afterwards, or your terminal
// will be left in a broken state.
func NewDebugger(cpu *cpu.Cpu, debugFile string) *Debugger {
	var symbols debugSymbols
	if len(debugFile) > 0 {
		var err error
		symbols, err = readDebugSymbols(debugFile)
		if err != nil {
			panic(err)
		}
	}

	liner := liner.NewLiner()
	liner.SetCompleter(linerCompleter(symbols))

	return &Debugger{
		liner:   liner,
		cpu:     cpu,
		symbols: symbols,
	}
}

// linerCompleter returns a tab-completion function for liner.
func linerCompleter(symbols debugSymbols) func(string) []string {
	return func(line string) (c []string) {
		if len(line) == 0 {
			return
		}

		// find index of current word being typed.
		i := len(line)
		for i > 0 && line[i-1] != ' ' {
			i--
		}
		prefix := line[:i]
		tail := line[i:]
		tailLower := strings.ToLower(tail)

		for _, l := range symbols.uniqueLabels() {
			if strings.HasPrefix(strings.ToLower(l), tailLower) {
				c = append(c, prefix+l)
			}
		}
		return
	}
}

// Shutdown the debugger session, including resetting the terminal to its previous
// state.
func (d *Debugger) Shutdown() {
	d.liner.Close()
}

// Queue a list of commands to be executed at the next prompt(s).
// This is useful for accepting a list of commands as a CLI parameter.
func (d *Debugger) QueueCommands(cmds []string) {
	d.inputQueue = append(d.inputQueue, cmds...)
}

func (d *Debugger) checkRegBreakpoint(regStr string, on bool, expect byte, actual byte) {
	if on && actual == expect {
		fmt.Printf("Breakpoint for %s = $%02X (%d)\n", regStr, expect, expect)
		d.run = false
	}
}

func (d *Debugger) doBreakpoints(in cpu.Instruction) {
	inName := in.Name()

	if inName == d.breakInstruction {
		fmt.Printf("Breakpoint for instruction %s\n", inName)
		d.run = false
	}

	if d.breakAddress && d.cpu.PC == d.breakAddressValue {
		fmt.Printf("Breakpoint for PC address = $%04X\n", d.breakAddressValue)
		d.run = false
	}

	d.checkRegBreakpoint("A", d.breakRegA, d.breakRegAValue, d.cpu.AC)
	d.checkRegBreakpoint("X", d.breakRegX, d.breakRegXValue, d.cpu.X)
	d.checkRegBreakpoint("Y", d.breakRegY, d.breakRegYValue, d.cpu.Y)
}

// BeforeExecute receives each cpu.Instruction just before the program
// counter is incremented and the instruction executed.
func (d *Debugger) BeforeExecute(in cpu.Instruction) {

	d.doBreakpoints(in)

	if d.run {
		return
	}

	fmt.Println(d.cpu)

	var symbols []string
	if in.IsAbsolute() {
		symbols = d.symbols.labelsFor(in.Op16)
	}

	if len(symbols) > 0 {
		fmt.Printf("Next: %v (%s)\n", in, strings.Join(symbols, ","))
	} else {
		fmt.Println("Next:", in)
	}

	for !d.commandLoop(in) {
		// next
	}
}

// Returns true when control is to be released.
func (d *Debugger) commandLoop(in cpu.Instruction) (release bool) {
	var (
		cmd *cmd
		err error
	)

	for cmd == nil && err == nil {
		cmd, err = d.getCommand()
	}
	if err != nil {
		panic(err)
	}

	switch cmd.id {
	case debugCmdBreakAddress:
		d.commandBreakAddress(cmd)
	case debugCmdBreakInstruction:
		d.breakInstruction = strings.ToUpper(cmd.arguments[0])
	case debugCmdBreakRegister:
		d.commandBreakRegister(cmd)
	case debugCmdContinue:
		d.run = true
		release = true
	case debugCmdExit:
		d.cpu.ExitChan <- 0
	case debugCmdHelp:
		d.commandHelp(cmd)
	case debugCmdNext:
		d.commandNext(in)
		release = true
	case debugCmdNone:
		// pass
	case debugCmdRead:
		d.commandRead(cmd)
	case debugCmdRead16:
		d.commandRead16(cmd)
	case debugCmdRead32:
		d.commandRead32(cmd)
	case debugCmdStep:
		release = true
	case debugCmdInvalid:
		fmt.Println("Invalid command.")
	default:
		panic("Unknown command code.")
	}

	return
}

// Set a breakpoint for the address after the current instruction, then
// continue execution. Steps over JSR, JMP etc. Probably doesn't do good
// things for branch instructions.
func (d *Debugger) commandNext(in cpu.Instruction) {
	addr := uint16(d.cpu.PC + uint16(in.Bytes))
	d.breakAddress = true
	d.breakAddressValue = addr
	d.run = true
}

func (d *Debugger) commandRead(cmd *cmd) {
	addr, err := d.parseUint16(cmd.arguments[0])
	if err != nil {
		panic(err)
	}
	v := d.cpu.Bus.Read(addr)
	fmt.Printf("$%04X => $%02X 0b%08b %d %q\n", addr, v, v, v, v)
}

func (d *Debugger) commandRead16(cmd *cmd) {
	addr, err := d.parseUint16(cmd.arguments[0])
	if err != nil {
		panic(err)
	}
	addrLo := addr
	addrHi := addr + 1
	vLo := uint16(d.cpu.Bus.Read(addrLo))
	vHi := uint16(d.cpu.Bus.Read(addrHi))
	v := vHi<<8 | vLo
	fmt.Printf("$%04X,%04X => $%04X 0b%016b %d\n", addrLo, addrHi, v, v, v)
}

func (d *Debugger) commandRead32(cmd *cmd) {
	addr, err := d.parseUint16(cmd.arguments[0])
	if err != nil {
		panic(err)
	}
	addr0 := addr
	addr1 := addr + 1
	addr2 := addr + 2
	addr3 := addr + 3
	v0 := uint32(d.cpu.Bus.Read(addr0))
	v1 := uint32(d.cpu.Bus.Read(addr1))
	v2 := uint32(d.cpu.Bus.Read(addr2))
	v3 := uint32(d.cpu.Bus.Read(addr3))
	v := v3<<24 | v2<<16 | v1<<8 | v0
	fmt.Printf("$%04X..%04X => $%08X 0b%032b %d\n", addr0, addr3, v, v, v)
}

func (d *Debugger) commandHelp(cmd *cmd) {
	fmt.Println("")
	fmt.Println("pda6502 debuger")
	fmt.Println("---------------")
	fmt.Println("break-address <addr> (alias: ba) e.g. ba 0x1000")
	fmt.Println("break-instruction <mnemonic> (alias: bi) e.g. bi NOP")
	fmt.Println("break-register <x|y|a> <value> (alias: br) e.g. br x 128")
	fmt.Println("continue (alias: c) Run continuously until breakpoint.")
	fmt.Println("exit (alias: quit, q) Shut down the emulator.")
	fmt.Println("help (alias: h, ?) This help.")
	fmt.Println("next (alias: n) Next instruction; step over subroutines.")
	fmt.Println("read <address> - Read and display 8-bit integer at address.")
	fmt.Println("read16 <address> - Read and display 16-bit integer at address.")
	fmt.Println("read32 <address> - Read and display 32-bit integer at address.")
	fmt.Println("step (alias: s) Run only the current instruction.")
	fmt.Println("(blank) Repeat the previous command.")
	fmt.Println("")
	fmt.Println("Hex input formats: 0x1234 $1234")
	fmt.Println("Commands expecting uint16 treat . as current address (PC).")
}

func (d *Debugger) commandBreakAddress(cmd *cmd) {
	addr, err := d.parseUint16(cmd.arguments[0])
	if err != nil {
		panic(err)
	}
	d.breakAddress = true
	d.breakAddressValue = addr
	fmt.Printf("break-address set to $%04X\n", addr)
}

func (d *Debugger) commandBreakRegister(cmd *cmd) {
	regStr := cmd.arguments[0]
	valueStr := cmd.arguments[1]

	var ptr *byte
	switch regStr {
	case "A", "a", "AC", "ac":
		d.breakRegA = true
		ptr = &d.breakRegAValue
	case "X", "x":
		d.breakRegX = true
		ptr = &d.breakRegXValue
	case "Y", "y":
		d.breakRegY = true
		ptr = &d.breakRegYValue
	default:
		panic(fmt.Errorf("Invalid register for break-register"))
	}

	value, err := d.parseUint8(valueStr)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Breakpoint set: %s = $%02X (%d)\n", regStr, value, value)

	*ptr = value
}

func (d *Debugger) getCommand() (*cmd, error) {
	var (
		id        int
		cmdString string
		arguments []string
		c         *cmd
		input     string
		err       error
	)

	if len(d.inputQueue) > 0 {
		input = d.inputQueue[0]
		d.inputQueue = d.inputQueue[1:]
		fmt.Printf("%s%s\n", d.prompt(), input)
	} else {
		input, err = d.readInput()
		if err != nil {
			return nil, err
		}
	}

	fields := strings.Fields(input)

	if len(fields) >= 1 {
		cmdString = strings.ToLower(fields[0])
	}
	if len(fields) >= 2 {
		arguments = fields[1:]
	}

	switch cmdString {
	case "":
		id = debugCmdNone
	case "break-address", "break-addr", "ba":
		id = debugCmdBreakAddress
	case "break-instruction", "bi":
		id = debugCmdBreakInstruction
	case "break-register", "break-reg", "br":
		id = debugCmdBreakRegister
	case "continue", "c":
		id = debugCmdContinue
	case "exit", "quit", "q":
		id = debugCmdExit
	case "help", "h", "?":
		id = debugCmdHelp
	case "next", "n":
		id = debugCmdNext
	case "read":
		id = debugCmdRead
	case "read16":
		id = debugCmdRead16
	case "read32":
		id = debugCmdRead32
	case "step", "st", "s":
		id = debugCmdStep
	default:
		id = debugCmdInvalid
	}

	if id == debugCmdNone && d.lastCmd != nil {
		c = d.lastCmd
	} else {
		c = &cmd{id, input, arguments}
		d.lastCmd = c
	}

	return c, nil
}

func (d *Debugger) readInput() (string, error) {
	input, err := d.liner.Prompt(d.prompt())
	if err != nil {
		return "", err
	}
	d.liner.AppendHistory(input)
	return input, nil
}

func (d *Debugger) prompt() string {
	symbols := strings.Join(d.symbols.labelsFor(d.cpu.PC), ",")
	return fmt.Sprintf("$%04X %s> ", d.cpu.PC, symbols)
}

func (d *Debugger) parseUint8(s string) (uint8, error) {
	s = strings.Replace(s, "$", "0x", 1)
	result, err := strconv.ParseUint(s, 0, 8)
	return uint8(result), err
}

func (d *Debugger) parseUint16(s string) (uint16, error) {
	if s == "." {
		return d.cpu.PC, nil
	}

	addresses := d.symbols.addressesFor(s)
	if len(addresses) == 1 {
		return addresses[0], nil
	} else if len(addresses) > 1 {
		// TODO: show addresses as hex, not dec, in error.
		// TODO: include addresses as []uint16 in error.
		return 0, fmt.Errorf("Multiple addresses for %s: %v", s, addresses)
	}

	s = strings.Replace(s, "$", "0x", 1)
	result, err := strconv.ParseUint(s, 0, 16)
	return uint16(result), err
}
