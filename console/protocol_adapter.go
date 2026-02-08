package console

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rwirdemann/modbuslabs/message"
	"golang.org/x/term"
)

type ProtocolAdapter struct {
	muted            bool
	loglevel         message.Type
	writer           io.Writer
	lastWasSeparator bool
}

func NewProtocolAdapter() *ProtocolAdapter {
	return &ProtocolAdapter{
		loglevel: message.TypeUnencoded,
		writer:   os.Stdout, // Default to stdout
	}
}

func (p *ProtocolAdapter) SetWriter(w io.Writer) {
	p.writer = w
}

func (p *ProtocolAdapter) InfoX(m message.Message) {
	if m.Type() == p.loglevel {
		p.lastWasSeparator = false
		ts := time.Now().Format(time.DateTime)
		p.print(fmt.Sprintf("%s %s", ts, m.String()), false)
	}
}

func (p *ProtocolAdapter) Toggle() {
	switch p.loglevel {
	case message.TypeEncoded:
		p.loglevel = message.TypeUnencoded
		p.Println("loglevel set to 'Unencoded'")
	case message.TypeUnencoded:
		p.loglevel = message.TypeEncoded
		p.Println("loglevel set to 'Encoded'")
	}
}

func (p *ProtocolAdapter) Info(msg string) {
	p.lastWasSeparator = false
	ts := time.Now().Format(time.DateTime)
	p.print(fmt.Sprintf("%s %s", ts, msg), false)
}

func (p *ProtocolAdapter) Separator() {
	if p.lastWasSeparator {
		return
	}
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		width = w
	}
	p.print(strings.Repeat("─", width), false)
	p.lastWasSeparator = true
}

func (p *ProtocolAdapter) ForceSeparator() {
	p.lastWasSeparator = false
	p.Separator()
}

func (p *ProtocolAdapter) Println(msg string) {
	p.lastWasSeparator = false
	p.print(msg, true)
}

func (p *ProtocolAdapter) Mute() {
	p.muted = true
}

func (p *ProtocolAdapter) Unmute() {
	p.muted = false
}

func (p *ProtocolAdapter) print(s string, force bool) {
	if !force && p.muted {
		return
	}

	fmt.Fprintln(p.writer, s)
}
