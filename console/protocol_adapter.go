package console

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// ProtocolAdapter implements ProtocolPort, writing to an io.Writer.
type ProtocolAdapter struct {
	muted            bool
	writer           io.Writer
	lastWasSeparator bool
}

// NewProtocolAdapter returns a ProtocolAdapter writing to stdout.
func NewProtocolAdapter() *ProtocolAdapter {
	return &ProtocolAdapter{writer: os.Stdout}
}

// SetWriter redirects output to w.
func (p *ProtocolAdapter) SetWriter(w io.Writer) {
	p.writer = w
}

// Info logs s with a timestamp unless muted.
func (p *ProtocolAdapter) Info(s string) {
	if p.muted {
		return
	}
	p.lastWasSeparator = false
	ts := time.Now().Format(time.DateTime)
	fmt.Fprintln(p.writer, ts+" "+s)
}

// Separator prints a horizontal rule, suppressed when preceded by one.
func (p *ProtocolAdapter) Separator() {
	if p.lastWasSeparator {
		return
	}
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		width = w
	}
	fmt.Fprintln(p.writer, strings.Repeat("─", width))
	p.lastWasSeparator = true
}

// ForceSeparator prints a horizontal rule unconditionally.
func (p *ProtocolAdapter) ForceSeparator() {
	p.lastWasSeparator = false
	p.Separator()
}

// Println writes msg regardless of muted state.
func (p *ProtocolAdapter) Println(msg string) {
	p.lastWasSeparator = false
	fmt.Fprintln(p.writer, msg)
}

// Mute suppresses Info output.
func (p *ProtocolAdapter) Mute() {
	p.muted = true
}

// Unmute re-enables Info output.
func (p *ProtocolAdapter) Unmute() {
	p.muted = false
}
