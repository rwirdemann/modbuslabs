package tcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/rwirdemann/modbuslabs"
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
	url      string
	listener net.Listener
}

func NewHandler(url string) (*Handler, error) {
	splitURL := strings.SplitN(url, "://", 2)
	if len(splitURL) == 2 {
		return &Handler{url: splitURL[1]}, nil
	}
	return nil, fmt.Errorf("invalid url format %s", url)
}

func (h *Handler) Start(ctx context.Context, cb modbuslabs.HandleMasterConnectionCallback) (err error) {
	h.listener, err = net.Listen("tcp", h.url)
	if err != nil {
		return fmt.Errorf("failed to start TCP listener: %w", err)
	}
	go h.acceptClients(ctx, cb)
	slog.Info("TCP listener started", "url", h.url)
	return nil
}

func (h *Handler) Stop() error {
	if h.listener != nil {
		slog.Info("Stopping TCP listener", "url", h.url)
		return h.listener.Close()
	}
	return nil
}

func (h *Handler) acceptClients(ctx context.Context, cb modbuslabs.HandleMasterConnectionCallback) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn, err := h.listener.Accept()
			if err == nil {
				slog.Info("client connected", "remote addr", conn.RemoteAddr())
				go cb(ctx, NewConnection(conn))
			}
		}
	}
}
