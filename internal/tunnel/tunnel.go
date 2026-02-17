package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/cedws/iapc/iap"
	"golang.org/x/oauth2"
)

// Tunnel represents an active IAP tunnel
type Tunnel struct {
	ID           string    `json:"id"`
	Project      string    `json:"project"`
	Zone         string    `json:"zone"`
	Instance     string    `json:"instance"`
	RemotePort   int       `json:"remotePort"`
	LocalPort    int       `json:"localPort"`
	Status       string    `json:"status"`
	StartedAt    time.Time `json:"startedAt"`
	BytesSent    int64     `json:"bytesSent"`
	BytesRecv    int64     `json:"bytesReceived"`
	Connections  int       `json:"connections"`
	ActiveConns  int       `json:"activeConnections"`

	listener      net.Listener
	cancelFunc    context.CancelFunc
	mu            sync.RWMutex
	lastActivity  time.Time
	idleTimer     *time.Timer
	autoClose     bool
	idleTimeout   time.Duration
}

// TunnelManager manages IAP tunnels
type TunnelManager struct {
	tokenSource  oauth2.TokenSource
	tunnels      map[string]*Tunnel
	mu           sync.RWMutex
	logChan      chan LogEntry
	autoClose    bool          // Auto-close tunnels when idle
	idleTimeout  time.Duration // How long to wait before closing idle tunnel
}

// LogEntry represents a tunnel log entry
type LogEntry struct {
	TunnelID  string    `json:"tunnelId"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

// NewTunnelManager creates a new tunnel manager
func NewTunnelManager(ts oauth2.TokenSource) *TunnelManager {
	return &TunnelManager{
		tokenSource: ts,
		tunnels:     make(map[string]*Tunnel),
		logChan:     make(chan LogEntry, 100),
		autoClose:   true,                // Auto-close idle tunnels by default
		idleTimeout: 2 * time.Minute,     // Close after 2 minutes of no connections
	}
}

// SetAutoClose configures auto-close behavior
func (tm *TunnelManager) SetAutoClose(enabled bool, timeout time.Duration) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.autoClose = enabled
	if timeout > 0 {
		tm.idleTimeout = timeout
	}
}

// SetTokenSource updates the token source
func (tm *TunnelManager) SetTokenSource(ts oauth2.TokenSource) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.tokenSource = ts
}

// StartTunnel starts a new IAP tunnel
func (tm *TunnelManager) StartTunnel(ctx context.Context, project, zone, instance string, remotePort int) (*Tunnel, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.tokenSource == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	// Find an available local port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}

	localPort := listener.Addr().(*net.TCPAddr).Port

	// Create tunnel context
	tunnelCtx, cancelFunc := context.WithCancel(ctx)

	tunnelID := fmt.Sprintf("%s-%s-%s-%d", project, zone, instance, remotePort)

	tunnel := &Tunnel{
		ID:         tunnelID,
		Project:    project,
		Zone:       zone,
		Instance:   instance,
		RemotePort: remotePort,
		LocalPort:  localPort,
		Status:     "running",
		StartedAt:  time.Now(),
		listener:   listener,
		cancelFunc: cancelFunc,
	}

	tm.tunnels[tunnelID] = tunnel

	// Start accepting connections
	go tm.acceptConnections(tunnelCtx, tunnel)

	tm.log(tunnelID, "info", fmt.Sprintf("Tunnel started on localhost:%d -> %s:%d", localPort, instance, remotePort))

	return tunnel, nil
}

// acceptConnections accepts incoming connections and forwards them through IAP
func (tm *TunnelManager) acceptConnections(ctx context.Context, t *Tunnel) {
	defer func() {
		t.mu.Lock()
		t.Status = "stopped"
		t.mu.Unlock()
		t.listener.Close()
		tm.log(t.ID, "info", "Tunnel stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Set accept deadline to allow checking context
		t.listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))

		conn, err := t.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			tm.log(t.ID, "error", fmt.Sprintf("Accept error: %v", err))
			return
		}

		t.mu.Lock()
		t.Connections++
		t.mu.Unlock()

		tm.log(t.ID, "info", fmt.Sprintf("New connection from %s", conn.RemoteAddr()))

		go tm.handleConnection(ctx, t, conn)
	}
}

// handleConnection handles a single tunnel connection
func (tm *TunnelManager) handleConnection(ctx context.Context, t *Tunnel, localConn net.Conn) {
	defer localConn.Close()

	tm.mu.RLock()
	ts := tm.tokenSource
	tm.mu.RUnlock()

	// Create IAP connection
	iapConn, err := iap.Dial(ctx,
		iap.WithProject(t.Project),
		iap.WithInstance(t.Instance, t.Zone, "nic0"),
		iap.WithPort(fmt.Sprintf("%d", t.RemotePort)),
		iap.WithTokenSource(&ts),
	)
	if err != nil {
		tm.log(t.ID, "error", fmt.Sprintf("Failed to dial IAP: %v", err))
		return
	}
	defer iapConn.Close()

	tm.log(t.ID, "info", "IAP connection established")

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	// Local -> IAP
	go func() {
		defer wg.Done()
		n, _ := io.Copy(iapConn, localConn)
		t.mu.Lock()
		t.BytesSent += n
		t.mu.Unlock()
	}()

	// IAP -> Local
	go func() {
		defer wg.Done()
		n, _ := io.Copy(localConn, iapConn)
		t.mu.Lock()
		t.BytesRecv += n
		t.mu.Unlock()
	}()

	wg.Wait()
	tm.log(t.ID, "info", "Connection closed")
}

// StopTunnel stops an active tunnel
func (tm *TunnelManager) StopTunnel(tunnelID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tunnel, exists := tm.tunnels[tunnelID]
	if !exists {
		return fmt.Errorf("tunnel not found: %s", tunnelID)
	}

	tunnel.cancelFunc()
	delete(tm.tunnels, tunnelID)

	return nil
}

// GetTunnel returns a specific tunnel
func (tm *TunnelManager) GetTunnel(tunnelID string) (*Tunnel, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tunnel, exists := tm.tunnels[tunnelID]
	if !exists {
		return nil, fmt.Errorf("tunnel not found: %s", tunnelID)
	}

	return tunnel, nil
}

// ListTunnels returns all active tunnels
func (tm *TunnelManager) ListTunnels() []*Tunnel {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tunnels := make([]*Tunnel, 0, len(tm.tunnels))
	for _, t := range tm.tunnels {
		tunnels = append(tunnels, t)
	}

	return tunnels
}

// GetTunnelForInstance returns a tunnel for a specific instance if one exists
func (tm *TunnelManager) GetTunnelForInstance(project, zone, instance string, remotePort int) *Tunnel {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tunnelID := fmt.Sprintf("%s-%s-%s-%d", project, zone, instance, remotePort)
	return tm.tunnels[tunnelID]
}

// StopAllTunnels stops all active tunnels
func (tm *TunnelManager) StopAllTunnels() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for id, tunnel := range tm.tunnels {
		tunnel.cancelFunc()
		delete(tm.tunnels, id)
	}
}

// GetLogChannel returns the log channel for receiving tunnel logs
func (tm *TunnelManager) GetLogChannel() <-chan LogEntry {
	return tm.logChan
}

// log sends a log entry to the log channel
func (tm *TunnelManager) log(tunnelID, level, message string) {
	select {
	case tm.logChan <- LogEntry{
		TunnelID:  tunnelID,
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}:
	default:
		// Channel full, drop the message
	}
}

// TunnelInfo returns a serializable tunnel info struct
func (t *Tunnel) TunnelInfo() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return map[string]interface{}{
		"id":            t.ID,
		"project":       t.Project,
		"zone":          t.Zone,
		"instance":      t.Instance,
		"remotePort":    t.RemotePort,
		"localPort":     t.LocalPort,
		"status":        t.Status,
		"startedAt":     t.StartedAt,
		"bytesSent":     t.BytesSent,
		"bytesReceived": t.BytesRecv,
		"connections":   t.Connections,
	}
}
