package modbuslabs

type ProtocolPort interface {
	Info(msg string)
	Println(msg string)
	Separator()
	Mute()
	Unmute()
}
