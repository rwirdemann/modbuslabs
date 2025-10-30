package modbuslabs

import "github.com/rwirdemann/modbuslabs/message"

type ProtocolPort interface {
	InfoX(m message.Message)
	Info(msg string)

	// Println logs the output even when it's muted
	Println(msg string)

	Separator()
	Mute()
	Unmute()
	Toggle()
}
