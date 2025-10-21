package console

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

type ProtocolAdapter struct {
	lastLine string
	muted    bool
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
