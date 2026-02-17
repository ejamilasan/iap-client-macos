package viewer

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nakagami/grdp"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var logFile *os.File

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	logDir := filepath.Join(homeDir, "Library", "Logs", "IAP Client")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return
	}
	logFile, err = os.OpenFile(
		filepath.Join(logDir, "rdpviewer.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600,
	)
	if err == nil {
		log.SetOutput(logFile)
		handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})
		slog.SetDefault(slog.New(handler))
	}
}

// RDPSession represents a single RDP connection
type RDPSession struct {
	id             string
	client         *grdp.RdpClient
	screen         *image.RGBA
	bitmapCh       chan []grdp.Bitmap
	stopCh         chan struct{}
	connected      bool
	loginComplete  bool // true after OnReady callback fires
	detectedWidth  int  // actual resolution detected from frames
	detectedHeight int
	mu             sync.Mutex
}

// RDPViewerService handles multiple embedded RDP sessions
type RDPViewerService struct {
	ctx      context.Context
	sessions map[string]*RDPSession
	mu       sync.RWMutex
	width    int
	height   int
	quality  int
}

// NewRDPViewerService creates a new RDP viewer service
func NewRDPViewerService() *RDPViewerService {
	return &RDPViewerService{
		sessions: make(map[string]*RDPSession),
		width:    1280,
		height:   800,
		quality:  95,
	}
}

// SetContext sets the Wails context for event emission
func (v *RDPViewerService) SetContext(ctx context.Context) {
	v.ctx = ctx
}

// SetResolution sets the viewer resolution for new sessions
func (v *RDPViewerService) SetResolution(width, height int) {
	v.width = width
	v.height = height
}

// SetQuality sets the JPEG quality (1-100) for new sessions
func (v *RDPViewerService) SetQuality(quality int) {
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}
	v.quality = quality
}

// ConnectSession establishes a new RDP session with the given ID using default resolution
func (v *RDPViewerService) ConnectSession(sessionId, host string, port int, username, password string) error {
	return v.ConnectSessionWithResolution(sessionId, host, port, username, password, v.width, v.height)
}

// ConnectSessionWithResolution establishes a new RDP session with a specific resolution
func (v *RDPViewerService) ConnectSessionWithResolution(sessionId, host string, port int, username, password string, width, height int) error {
	log.Printf("[RDPViewer:%s] Connecting to %s:%d as %s (resolution: %dx%d)", sessionId, host, port, username, width, height)

	// Check if session already exists
	v.mu.Lock()
	if existing, ok := v.sessions[sessionId]; ok {
		delete(v.sessions, sessionId)
		v.mu.Unlock()
		// Clean up the existing session directly (no re-lock needed)
		existing.mu.Lock()
		if existing.stopCh != nil {
			close(existing.stopCh)
			existing.stopCh = nil
		}
		if existing.client != nil {
			existing.client.Close()
			existing.client = nil
		}
		existing.connected = false
		existing.screen = nil
		existing.bitmapCh = nil
		existing.mu.Unlock()
		log.Printf("[RDPViewer:%s] Closed existing session", sessionId)
	} else {
		v.mu.Unlock()
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client := grdp.NewRdpClient(addr, width, height)

	// Initialize screen buffer with black background
	screen := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(screen.Pix); i += 4 {
		screen.Pix[i] = 0     // R
		screen.Pix[i+1] = 0   // G
		screen.Pix[i+2] = 0   // B
		screen.Pix[i+3] = 255 // A (fully opaque)
	}

	// Create channels for bitmap processing
	bitmapCh := make(chan []grdp.Bitmap, 500)
	stopCh := make(chan struct{})

	// Create session
	session := &RDPSession{
		id:        sessionId,
		client:    client,
		screen:    screen,
		bitmapCh:  bitmapCh,
		stopCh:    stopCh,
		connected: false,
	}

	log.Printf("[RDPViewer:%s] Attempting login...", sessionId)

	// Login synchronously
	err := client.Login("", username, password)
	if err != nil {
		log.Printf("[RDPViewer:%s] Login failed: %v", sessionId, err)
		if v.ctx != nil {
			runtime.EventsEmit(v.ctx, fmt.Sprintf("rdp-error-%s", sessionId), err.Error())
		}
		return fmt.Errorf("RDP login failed: %w", err)
	}
	log.Printf("[RDPViewer:%s] Login successful, setting up callbacks", sessionId)

	// Set up callbacks with session-specific event names
	client.OnBitmap(func(bitmaps []grdp.Bitmap) {
		log.Printf("[RDPViewer:%s] Received %d bitmap updates", sessionId, len(bitmaps))
		select {
		case bitmapCh <- bitmaps:
		default:
			log.Printf("[RDPViewer:%s] Bitmap channel full, dropping frame", sessionId)
		}
	}).OnReady(func() {
		log.Printf("[RDPViewer:%s] Connection ready!", sessionId)
		session.mu.Lock()
		session.connected = true
		session.loginComplete = true
		session.mu.Unlock()
		if v.ctx != nil {
			runtime.EventsEmit(v.ctx, fmt.Sprintf("rdp-connected-%s", sessionId), true)
		}
	}).OnError(func(e error) {
		log.Printf("[RDPViewer:%s] Error: %v", sessionId, e)
		session.mu.Lock()
		wasConnected := session.connected
		wasLoginComplete := session.loginComplete
		session.connected = false
		session.mu.Unlock()

		// Check if this is a server-side disconnect (not a real error)
		errStr := e.Error()
		isServerDisconnect := strings.Contains(errStr, "use of closed network connection") ||
			strings.Contains(errStr, "DISCONNECT_PROVIDER_ULTIMATUM") ||
			strings.Contains(errStr, "connection reset by peer") ||
			strings.Contains(errStr, "EOF")

		// Only emit errors if:
		// 1. We were previously connected (post-login error/disconnect)
		// 2. OR this is NOT a transient connection error during login
		if v.ctx != nil {
			if wasLoginComplete {
				if isServerDisconnect && wasConnected {
					// Treat as a clean disconnect if we were previously connected
					runtime.EventsEmit(v.ctx, fmt.Sprintf("rdp-disconnected-%s", sessionId), true)
				} else {
					runtime.EventsEmit(v.ctx, fmt.Sprintf("rdp-error-%s", sessionId), e.Error())
				}
			} else {
				// During login phase, only log but don't emit to frontend
				// (grdp retries internally and these are transient)
				log.Printf("[RDPViewer:%s] Ignoring transient error during login: %v", sessionId, e)
			}
		}
	}).OnClose(func() {
		log.Printf("[RDPViewer:%s] Connection closed", sessionId)
		session.mu.Lock()
		session.connected = false
		session.mu.Unlock()
		if v.ctx != nil {
			runtime.EventsEmit(v.ctx, fmt.Sprintf("rdp-disconnected-%s", sessionId), true)
		}
	})

	// Store session
	v.mu.Lock()
	v.sessions[sessionId] = session
	v.mu.Unlock()

	// Start bitmap processing goroutine for this session
	go v.processSessionBitmaps(session)

	return nil
}

// processSessionBitmaps handles bitmap updates for a specific session
func (v *RDPViewerService) processSessionBitmaps(session *RDPSession) {
	for {
		select {
		case <-session.stopCh:
			return
		case bitmaps := <-session.bitmapCh:
			v.handleSessionBitmaps(session, bitmaps)
		}
	}
}

// handleSessionBitmaps processes incoming bitmap updates for a session
func (v *RDPViewerService) handleSessionBitmaps(session *RDPSession, bitmaps []grdp.Bitmap) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.screen == nil {
		log.Printf("[RDPViewer:%s] screen is nil, skipping bitmap update", session.id)
		return
	}

	// Detect actual resolution from bitmap coordinates
	for _, bm := range bitmaps {
		// DestRight and DestBottom give us the maximum extents
		if bm.DestRight > session.detectedWidth {
			session.detectedWidth = bm.DestRight
		}
		if bm.DestBottom > session.detectedHeight {
			session.detectedHeight = bm.DestBottom
		}
	}

	// Check if we need to resize the screen buffer
	currentBounds := session.screen.Bounds()
	if session.detectedWidth > currentBounds.Dx() || session.detectedHeight > currentBounds.Dy() {
		newWidth := session.detectedWidth
		newHeight := session.detectedHeight
		log.Printf("[RDPViewer:%s] Resizing screen buffer from %dx%d to %dx%d",
			session.id, currentBounds.Dx(), currentBounds.Dy(), newWidth, newHeight)

		// Create new larger buffer and copy existing content
		newScreen := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
		// Fill with black
		for i := 0; i < len(newScreen.Pix); i += 4 {
			newScreen.Pix[i] = 0
			newScreen.Pix[i+1] = 0
			newScreen.Pix[i+2] = 0
			newScreen.Pix[i+3] = 255
		}
		// Copy old content
		draw.Draw(newScreen, currentBounds, session.screen, image.Point{}, draw.Src)
		session.screen = newScreen

		// Emit resolution change event to frontend
		if v.ctx != nil {
			runtime.EventsEmit(v.ctx, fmt.Sprintf("rdp-resolution-%s", session.id), map[string]int{
				"width":  newWidth,
				"height": newHeight,
			})
		}
	}

	for _, bm := range bitmaps {
		img := bm.RGBA()
		if img == nil {
			continue
		}

		// Draw bitmap at its destination position on screen buffer
		destRect := session.screen.Bounds().Add(image.Pt(bm.DestLeft, bm.DestTop))
		draw.Draw(session.screen, destRect, img, img.Bounds().Min, draw.Src)
	}

	// Encode and emit frame with session-specific event
	v.emitSessionFrame(session)
}

// emitSessionFrame encodes the current screen buffer and emits it to the frontend
func (v *RDPViewerService) emitSessionFrame(session *RDPSession) {
	if v.ctx == nil {
		return
	}
	if session.screen == nil {
		return
	}

	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, session.screen, &jpeg.Options{Quality: v.quality})
	if err != nil {
		log.Printf("[RDPViewer:%s] emitFrame: JPEG encode error: %v", session.id, err)
		return
	}

	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	runtime.EventsEmit(v.ctx, fmt.Sprintf("rdp-frame-%s", session.id), b64)
}

// SendMouseMove forwards mouse movement to a specific session
func (v *RDPViewerService) SendMouseMove(sessionId string, x, y int) {
	v.mu.RLock()
	session, ok := v.sessions[sessionId]
	v.mu.RUnlock()

	if !ok {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.client != nil && session.connected {
		session.client.MouseMove(x, y)
	}
}

// SendMouseClick forwards mouse click to a specific session
func (v *RDPViewerService) SendMouseClick(sessionId string, x, y, button int, pressed bool) {
	v.mu.RLock()
	session, ok := v.sessions[sessionId]
	v.mu.RUnlock()

	if !ok {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.client != nil && session.connected {
		if pressed {
			session.client.MouseDown(button, x, y)
		} else {
			session.client.MouseUp(button, x, y)
		}
	}
}

// SendMouseWheel forwards mouse wheel to a specific session
func (v *RDPViewerService) SendMouseWheel(sessionId string, scroll int) {
	v.mu.RLock()
	session, ok := v.sessions[sessionId]
	v.mu.RUnlock()

	if !ok {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.client != nil && session.connected {
		session.client.MouseWheel(scroll)
	}
}

// SendKeyboard forwards keyboard input to a specific session
func (v *RDPViewerService) SendKeyboard(sessionId string, keyCode int, pressed bool) {
	v.mu.RLock()
	session, ok := v.sessions[sessionId]
	v.mu.RUnlock()

	if !ok {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.client != nil && session.connected {
		if pressed {
			session.client.KeyDown(keyCode)
		} else {
			session.client.KeyUp(keyCode)
		}
	}
}

// DisconnectSession closes a specific RDP session
func (v *RDPViewerService) DisconnectSession(sessionId string) {
	v.mu.Lock()
	session, ok := v.sessions[sessionId]
	if !ok {
		v.mu.Unlock()
		return
	}
	delete(v.sessions, sessionId)
	v.mu.Unlock()

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.stopCh != nil {
		close(session.stopCh)
		session.stopCh = nil
	}
	if session.client != nil {
		session.client.Close()
		session.client = nil
	}
	session.connected = false
	session.screen = nil
	session.bitmapCh = nil

	log.Printf("[RDPViewer:%s] Session disconnected", sessionId)
}

// DisconnectAll closes all RDP sessions
func (v *RDPViewerService) DisconnectAll() {
	v.mu.Lock()
	sessionIds := make([]string, 0, len(v.sessions))
	for id := range v.sessions {
		sessionIds = append(sessionIds, id)
	}
	v.mu.Unlock()

	for _, id := range sessionIds {
		v.DisconnectSession(id)
	}
}

// IsSessionConnected returns whether a specific session is connected
func (v *RDPViewerService) IsSessionConnected(sessionId string) bool {
	v.mu.RLock()
	session, ok := v.sessions[sessionId]
	v.mu.RUnlock()

	if !ok {
		return false
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	return session.connected
}

// GetSessionIds returns a list of all active session IDs
func (v *RDPViewerService) GetSessionIds() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	ids := make([]string, 0, len(v.sessions))
	for id := range v.sessions {
		ids = append(ids, id)
	}
	return ids
}

// GetResolution returns the current resolution
func (v *RDPViewerService) GetResolution() (int, int) {
	return v.width, v.height
}

// ============================================================================
// Backward compatibility methods (for single session use)
// ============================================================================

// Connect establishes an RDP connection (backward compatible - uses "default" session)
func (v *RDPViewerService) Connect(host string, port int, username, password string) error {
	return v.ConnectSession("default", host, port, username, password)
}

// Disconnect closes the default RDP connection
func (v *RDPViewerService) Disconnect() {
	v.DisconnectSession("default")
}

// IsConnected returns whether the default session is connected
func (v *RDPViewerService) IsConnected() bool {
	return v.IsSessionConnected("default")
}
