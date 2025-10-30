package message

type Unencoded struct {
	Value string
}

func NewUnencoded(value string) Unencoded {
	return Unencoded{Value: value}
}

func (m Unencoded) String() string {
	return m.Value
}

func (m Unencoded) Type() Type {
	return TypeUnencoded
}
