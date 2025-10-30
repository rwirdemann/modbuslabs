package message

type Encoded struct {
	Value string
}

func NewEncoded(value string) Encoded {
	return Encoded{Value: value}
}

func (m Encoded) String() string {
	return m.Value
}

func (m Encoded) Type() Type {
	return TypeEncoded
}
