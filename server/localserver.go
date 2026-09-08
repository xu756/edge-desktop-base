package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const maxWebSocketMessageSize = 1 << 20

type LocalServerStatus struct {
	Address string `json:"address"`
	Running bool   `json:"running"`
}

type LocalServer struct {
	mu       sync.RWMutex
	address  string
	running  bool
	listener net.Listener
	server   *http.Server
}

func NewLocalServer(address string) *LocalServer {
	return &LocalServer{address: address}
}

func (s *LocalServer) ServiceStartup(context.Context, application.ServiceOptions) error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "version": Version})
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

func (s *LocalServer) ServiceShutdown() error {
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

func (s *LocalServer) Status() LocalServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return LocalServerStatus{Address: s.address, Running: s.running}
}

func (s *LocalServer) WebSocketURL() string {
	return "ws://" + s.address + "/ws"
}

func (s *LocalServer) handleWebSocket(c *gin.Context) {
	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		OriginPatterns: []string{
			"wails.localhost",
			"wails.localhost:*",
			"localhost",
			"localhost:*",
			"127.0.0.1",
			"127.0.0.1:*",
		},
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "closed")
	conn.SetReadLimit(maxWebSocketMessageSize)

	ctx := c.Request.Context()
	if err := wsjson.Write(ctx, conn, map[string]any{
		"type":    "runtime.ready",
		"payload": map[string]any{"version": Version},
	}); err != nil {
		return
	}
	for {
		var message any
		if err := wsjson.Read(ctx, conn, &message); err != nil {
			return
		}
		if err := wsjson.Write(ctx, conn, map[string]any{"type": "echo", "payload": message}); err != nil {
			return
		}
	}
}
