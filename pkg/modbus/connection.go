package modbus

type Connection interface {
	Read(p []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	Close()
	Name() string
}
