package modbuslabs

type ProtocolPort interface {
	Info(s string)

	// Println logs the output even when it's muted
	Println(msg string)

	Separator()
	ForceSeparator()
	Mute()
	Unmute()
}
