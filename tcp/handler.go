package tcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"github.com/rwirdemann/modbuslabs"
	"github.com/rwirdemann/modbuslabs/pkg/modbus"
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

func (h *Handler) startRequestCycle(ctx context.Context, processPDU modbuslabs.ProcessPDUCallback) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			slog.Debug("listening...")
			conn, err := h.listener.Accept()
			if err == nil {
				slog.Debug("client connected", "remote addr", conn.RemoteAddr())
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
	header, pdu, txnId, err := modbus.ReadMBAPFrame(conn)
	if err != nil {
		if err == io.EOF {
			slog.Debug("client disconnected", "remote addr", conn.RemoteAddr())
			conn.Close()
		}
		return err
	}
	slog.Debug("MBAP header received", "pdu", pdu, "txid", txnId)

	h.protocolPort.Separator()
	h.protocolPort.Info(fmt.Sprintf("TX % X % X % X", header, pdu.FunctionCode, pdu.Payload))

	res := processPDU(*pdu)

	if res != nil {
		payload := modbus.AssembleMBAPFrame(txnId, res)
		if _, err := conn.Write(payload); err != nil {
			return err
		}
		slog.Debug(fmt.Sprintf("MBAP response written: % X", payload))
		h.protocolPort.Info(fmt.Sprintf("RX % X", payload))
	}
	h.protocolPort.Separator()
	return nil
}
