package modbus

type Reader interface {
	Read(p []byte) (n int, err error)
	Close()
	Name() string
}
