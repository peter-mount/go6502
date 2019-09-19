package acia6551

import "os"

// A terminal attached to a acia6551 - in this case this is either stdin & stdout
// But can also be any 2 files
type Terminal struct {
	console bool
	in      *os.File
	out     *os.File
}

func (t *Terminal) Capabilities() int {
	if t.in != nil && t.out != nil {
		return BiDirectional
	}
	if t.in != nil {
		return Read
	}
	if t.out != nil {
		return Write
	}
	return Nop
}

func (t *Terminal) Read() (bool, byte, error) {
	// No input device
	if t.in == nil {
		return false, 0, nil
	}

	b := make([]byte, 1)
	n, err := t.in.Read(b)
	if err != nil {
		return false, 0, err
	}

	return n == 1, b[0], nil
}

func (t *Terminal) Write(b byte) (bool, error) {
	// No output device
	if t.out == nil {
		return false, nil
	}

	n, err := t.out.Write([]byte{b})
	return n == 1, err
}

func (t *Terminal) Shutdown() {
	if !t.console {
		if t.in != nil {
			_ = t.in.Close()
		}

		if t.out != nil {
			_ = t.out.Close()
		}
	}
}

// NewConsole returns a Terminal attached to the console
func NewConsole() *Terminal {
	return &Terminal{
		console: true,
		in:      os.Stdin,
		out:     os.Stdout,
	}
}

// NewTerminal returns a Terminal attached to 2 files
func NewTerminal(in *os.File, out *os.File) *Terminal {
	return &Terminal{
		console: false,
		in:      in,
		out:     out,
	}
}
