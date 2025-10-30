package console

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rwirdemann/modbuslabs/message"
	"golang.org/x/term"
)

type ProtocolAdapter struct {
	lastLine string
	muted    bool
	loglevel message.Type
}

func NewProtocolAdapter() *ProtocolAdapter {
	return &ProtocolAdapter{loglevel: message.TypeUnencoded}
}

func (p *ProtocolAdapter) InfoX(m message.Message) {
	if m.Type() == p.loglevel {
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
	ts := time.Now().Format(time.DateTime)
	p.print(fmt.Sprintf("%s %s", ts, msg), false)
}

func (p *ProtocolAdapter) Separator() {
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		width = w
	}
	p.print(strings.Repeat("─", width), false)
}

func (p *ProtocolAdapter) Println(msg string) {
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

	if p.lastLine == s {
		return
	}
	fmt.Println(s)
	p.lastLine = s
}
