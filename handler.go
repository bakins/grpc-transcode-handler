package handler

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"sync"

	"github.com/akutz/memconn"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ErrorLogger handles HTTP error logging
type ErrorLogger interface {
	Log(message string, err error)
}

// Handler is a transcoding HTTP+JSON to gRPC HTTP handler
type Handler struct {
	server        *grpc.Server
	client        *grpc.ClientConn
	listener      net.Listener
	logger        ErrorLogger
	clientOptions []grpc.DialOption
	serverOptions []grpc.ServerOption
	address       string
}

// Option is passed to New to set options on the Handler
type Option func(*Handler)

var (
	idMutex   sync.Mutex
	currentID uint64
)

type empty struct{}

func generateID() string {
	idMutex.Lock()
	defer idMutex.Unlock()
	pkg := reflect.TypeOf(empty{}).PkgPath()
	currentID++
	return fmt.Sprintf("%s-%d", pkg, currentID)
}

// New creates a transcoding Handler.
func New(options ...Option) (*Handler, error) {
	addr := memconn.Addr{Name: generateID()}
	lis, err := memconn.ListenMem("memu", &addr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create in-memory listener")
	}

	h := &Handler{
		listener: lis,
		logger:   nullLogger{},
	}

	for _, o := range options {
		o(h)
	}

	serverOptions := append(h.serverOptions, grpc.CustomCodec(codec()))

	h.server = grpc.NewServer(serverOptions...)

	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return memconn.DialMemContext(ctx, "memu", nil, &addr)
	}

	dialOptions := []grpc.DialOption{
		grpc.WithContextDialer(dialer),
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(
			grpc.ForceCodec(codec()),
		),
	}

	dialOptions = append(h.clientOptions, dialOptions...)
	conn, err := grpc.Dial("", dialOptions...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create grpc client")
	}

	h.client = conn

	return h, nil
}

// WithClientOptions sets additional gRPC Dial options.
func WithClientOptions(options ...grpc.DialOption) Option {
	return func(h *Handler) {
		opts := make([]grpc.DialOption, len(options))
		copy(opts, options)
		h.clientOptions = opts
	}
}

// WithServerOptions sets additional gRPC Server options.
func WithServerOptions(options ...grpc.ServerOption) Option {
	return func(h *Handler) {
		opts := make([]grpc.ServerOption, len(options))
		copy(opts, options)
		h.serverOptions = opts
	}
}

// WithErrorLogger sets the ErrorLogger
func WithErrorLogger(e ErrorLogger) Option {
	return func(h *Handler) {
		h.logger = e
	}
}

// Server returns the internal gRPC server
func (h *Handler) Server() *grpc.Server {
	return h.server
}

// Start starts  the internal gRPC server. This generally does not return.
// See https://godoc.org/google.golang.org/grpc#Server.Serve
func (h *Handler) Start() error {
	return h.server.Serve(h.listener)
}

// Stop stops the gRPC server.
// See https://godoc.org/google.golang.org/grpc#Server.Stop
func (h *Handler) Stop() {
	h.server.Stop()
}

// GracefulStop stops the gRPC server gracefully.
// See https://godoc.org/google.golang.org/grpc#Server.GracefulStop
func (h *Handler) GracefulStop() {
	h.server.GracefulStop()
}

func (h *Handler) handlerError(w http.ResponseWriter, message string, err error, code int) {
	h.logger.Log(message, err)
	http.Error(w, err.Error(), code)
}

// ServeHTTP handles HTTP+JSON requests and proxies to an internal gRPC server
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.handlerError(w, "failed to read request body", err, http.StatusBadRequest)
		return
	}

	desc := grpc.StreamDesc{
		StreamName:    "transcode",
		ClientStreams: false,
		ServerStreams: true,
	}

	requestMetadata := metadata.New(nil)
	for k, v := range r.Header {
		requestMetadata.Set(k, v...)
	}
	requestMetadata.Set("Content-Type", "application/grpc+json")

	ctx := metadata.NewOutgoingContext(r.Context(), requestMetadata)

	stream, err := h.client.NewStream(ctx, &desc, r.URL.Path)
	if err != nil {
		h.handlerError(w, "failed to create client stream", err, http.StatusInternalServerError)
		return
	}

	if err := stream.SendMsg(&frame{payload: body}); err != nil {
		h.handlerError(w, "failed to send grpc message", err, http.StatusBadGateway)
		return
	}

	if err := stream.CloseSend(); err != nil {
		h.handlerError(w, "failed to close grpc send channel", err, http.StatusBadGateway)
		return
	}

	md, err := stream.Header()
	if err != nil {
		h.handlerError(w, "failed to recieve response headers", err, http.StatusBadGateway)
		return
	}

	headers := w.Header()
	for k, v := range md {
		for _, val := range v {
			headers.Add(k, val)
		}
	}

	headers.Set("Content-Type", "application/json")

	for {
		var f frame
		err := stream.RecvMsg(&f)
		if err == io.EOF {
			break
		}
		if err != nil {
			h.handlerError(w, "failed to receive grpc message", err, http.StatusBadGateway)
			return
		}

		if _, err := w.Write(f.payload); err != nil {
			h.handlerError(w, "failed to send response", err, http.StatusInternalServerError)
			return
		}
		// response streams are seperated by newlines
		_, _ = w.Write([]byte("\n"))
	}
}

type nullLogger struct{}

func (n nullLogger) Log(message string, err error) {}
