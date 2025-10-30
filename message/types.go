package message

type Type int

const (
	TypeUnencoded Type = iota
	TypeEncoded
)

type Message interface {
	String() string
	Type() Type
}
