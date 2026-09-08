package localapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const maxWebSocketMessageSize = 1 << 20

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Status struct {
	Address string `json:"address"`
	Running bool   `json:"running"`
}

type Server struct {
	mu       sync.RWMutex
	address  string
	version  string
	running  bool
	listener net.Listener
	server   *http.Server
}

func New(address, version string) *Server {
	return &Server{address: address, version: version}
}

func (s *Server) ServiceStartup(context.Context, application.ServiceOptions) error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "version": s.version})
	})
	router.GET("/ws", s.handleWebSocket)

	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("listen local server %s: %w", s.address, err)
	}
	httpServer := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	s.mu.Lock()
	s.listener = listener
	s.server = httpServer
	s.running = true
	s.mu.Unlock()

	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("local server stopped: %v", err)
		}
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()
	return nil
}

func (s *Server) ServiceShutdown() error {
	s.mu.RLock()
	httpServer := s.server
	s.mu.RUnlock()
	if httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return httpServer.Shutdown(ctx)
}

func (s *Server) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	address := s.address
	if s.listener != nil {
		address = s.listener.Addr().String()
	}
	return Status{Address: address, Running: s.running}
}

func (s *Server) WebSocketURL() string {
	return "ws://" + s.Status().Address + "/ws"
}

func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()
	conn.SetReadLimit(maxWebSocketMessageSize)

	if err := conn.WriteJSON(map[string]any{
		"type":    "runtime.ready",
		"payload": map[string]any{"version": s.version},
	}); err != nil {
		return
	}
	for {
		var message interface{}
		err := conn.ReadJSON(&message)
		if err != nil {
			break
		}
		if err := conn.WriteJSON(map[string]any{"type": "echo", "payload": message}); err != nil {
			return
		}
	}
}
