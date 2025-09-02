package console

import (
	"fmt"
	"time"
)

type ProtocolAdapter struct {
}

func (p ProtocolAdapter) Info(msg string) {
	ts := time.Now().Format(time.DateTime)
	fmt.Printf("%s: %s\n", ts, msg)
}
