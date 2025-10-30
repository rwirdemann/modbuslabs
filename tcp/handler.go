package tcp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"github.com/rwirdemann/modbuslabs"
	"github.com/rwirdemann/modbuslabs/message"
	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

const (
	MBAPHeaderLength = 7
	MaxFrameLength   = 260
)

type Connection struct {
	conn net.Conn
}

func NewConnection(c net.Conn) *Connection {
	return &Connection{conn: c}
}

func (r Connection) Read(p []byte) (n int, err error) {
	return r.conn.Read(p)
}

func (r Connection) Write(b []byte) (n int, err error) {
	return r.conn.Write(b)
}

func (r Connection) Close() {
	r.conn.Close()
}

func (r Connection) Name() string {
	return r.conn.RemoteAddr().String()
}

type Handler struct {
	url          string
	listener     net.Listener
	protocolPort modbuslabs.ProtocolPort
}

func NewHandler(url string, protocolPort modbuslabs.ProtocolPort) (*Handler, error) {
	splitURL := strings.SplitN(url, "://", 2)
	if len(splitURL) == 2 {
		return &Handler{url: splitURL[1], protocolPort: protocolPort}, nil
	}
	return nil, fmt.Errorf("invalid url format %s", url)
}

func (h *Handler) Start(ctx context.Context, processPDU modbuslabs.ProcessPDUCallback) (err error) {
	h.listener, err = net.Listen("tcp", h.url)
	if err != nil {
		return fmt.Errorf("failed to start TCP listener: %w", err)
	}
	go h.startRequestCycle(ctx, processPDU)
	slog.Debug("TCP listener started", "url", h.url)
	return nil
}

func (h *Handler) Stop() error {
	if h.listener != nil {
		slog.Debug("Stopping TCP listener", "url", h.url)
		return h.listener.Close()
	}
	return nil
}

func (h *Handler) Description() string {
	return h.url
}

func (h *Handler) startRequestCycle(ctx context.Context, processPDU modbuslabs.ProcessPDUCallback) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			slog.Debug("listening...")
			conn, err := h.listener.Accept()
			if err == nil {
				go func() {
					for {
						if err := h.processRequest(conn, processPDU); err != nil {
							break
						}
					}
				}()
			}
		}
	}
}

func (h *Handler) processRequest(conn net.Conn, processPDU modbuslabs.ProcessPDUCallback) error {
	header, pdu, txnId, err := readMBAPFrame(conn)
	if err != nil {
		if err == io.EOF {
			slog.Debug("client disconnected", "remote addr", conn.RemoteAddr())
			conn.Close()
		}
		return err
	}
	slog.Debug("MBAP header received", "pdu", pdu, "txid", txnId)

	h.protocolPort.Separator()
	m := message.Unencoded{Value: fmt.Sprintf("TX % X % X % X", header, pdu.FunctionCode, pdu.Payload)}
	h.protocolPort.InfoX(m)

	res := processPDU(*pdu)

	if res != nil {
		payload := modbus.AssembleMBAPFrame(txnId, res)
		if _, err := conn.Write(payload); err != nil {
			return err
		}
		slog.Debug(fmt.Sprintf("MBAP response written: % X", payload))
		h.protocolPort.InfoX(message.NewUnencoded(fmt.Sprintf("RX % X", payload)))
	}
	h.protocolPort.Separator()
	return nil
}

// readMBAPFrame reads an entire frame (MBAP header + modbus PDU) from the reader. Example:
//
// 00 01 00 00 00 06 01 03 00 01 00 02
// 00 01      - Transaction ID (1)
// 00 00      - Protocol ID (0 = Modbus)
// 00 06      - Length (6 Bytes folgen)
// 01         - Unit ID (Slave 1)
// 03         - Function Code (Read Holding Registers)
// 00 01      - Start Address (Register 1)
// 00 02      - Quantity (2 Register)
//
// Returns the header, [PDU] and transaction id on success.
func readMBAPFrame(conn io.Reader) ([]byte, *modbus.PDU, uint16, error) {

	// read the MBAP header
	header := make([]byte, MBAPHeaderLength)
	_, err := io.ReadFull(conn, header)
	if err != nil {
		return nil, nil, 0, err
	}

	// decode the transaction identifier as unique request and response identifier
	txid := binary.BigEndian.Uint16(header[0:2])

	// decode the protocol identifier
	protocolId := binary.BigEndian.Uint16(header[2:4])

	// store the source unit id
	unitId := header[6]

	// determine how many more bytes we need to read
	bytesNeeded := binary.BigEndian.Uint16(header[4:6])

	// the byte count includes the unit ID field, which we already have
	bytesNeeded--

	// never read more than the max allowed frame length
	if int(bytesNeeded)+MBAPHeaderLength > MaxFrameLength {
		return nil, nil, 0, errors.New("protocol error: maxFrameLength exceeded")
	}

	// an MBAP length of 0 is illegal
	if bytesNeeded <= 0 {
		return nil, nil, 0, errors.New("protocol error: length is equal or less 0")
	}

	// read the PDU
	rxbuf := make([]byte, bytesNeeded)
	_, err = io.ReadFull(conn, rxbuf)
	if err != nil {
		return nil, nil, 0, err
	}

	// validate protocol id
	if protocolId != 0x0000 {
		return nil, nil, 0, errors.New("protocol error: invalid protocol id")
	}

	// store unit id, function code and payload in the PDU object
	pdu := &modbus.PDU{
		UnitId:       unitId,
		FunctionCode: rxbuf[0],
		Payload:      rxbuf[1:],
	}

	return header, pdu, txid, nil
}
