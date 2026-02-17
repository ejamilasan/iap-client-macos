package main

import (
	"context"
	"fmt"

	"iap-client-macos/internal/auth"
	"iap-client-macos/internal/config"
	"iap-client-macos/internal/gcp"
	"iap-client-macos/internal/keychain"
	"iap-client-macos/internal/rdp"
	"iap-client-macos/internal/tunnel"
	"iap-client-macos/internal/viewer"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds all the application services
type App struct {
	ctx             context.Context
	authService     *auth.AuthService
	projectService  *gcp.ProjectService
	instanceService *gcp.InstanceService
	tunnelManager   *tunnel.TunnelManager
	configService   *config.ConfigService
	keychainService *keychain.KeychainService
	rdpService      *rdp.RDPService
	passwordService *rdp.PasswordService
	viewerService   *viewer.RDPViewerService
}

// NewApp creates a new App application struct
func NewApp() *App {
	authSvc := auth.NewAuthService()
	configSvc, err := config.NewConfigService()
	if err != nil {
		fmt.Printf("Warning: failed to initialize config service: %v\n", err)
	}

	return &App{
		authService:     authSvc,
		configService:   configSvc,
		keychainService: keychain.NewKeychainService(),
		rdpService:      rdp.NewRDPService(),
		viewerService:   viewer.NewRDPViewerService(),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Set context for viewer service
	if a.viewerService != nil {
		a.viewerService.SetContext(ctx)
	}

	// Initialize services that depend on auth
	if a.authService.IsAuthenticated() {
		a.initializeAuthenticatedServices()
	}
}

// initializeAuthenticatedServices sets up services that require authentication
func (a *App) initializeAuthenticatedServices() {
	ts := a.authService.GetTokenSource()
	if ts == nil {
		return
	}

	a.projectService = gcp.NewProjectService(ts)
	a.instanceService = gcp.NewInstanceService(ts)
	a.tunnelManager = tunnel.NewTunnelManager(ts)
	a.passwordService = rdp.NewPasswordService(ts)
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	// Disconnect viewer if connected
	if a.viewerService != nil {
		a.viewerService.Disconnect()
	}
	if a.tunnelManager != nil {
		a.tunnelManager.StopAllTunnels()
	}
	if a.rdpService != nil {
		a.rdpService.CleanupTempFiles()
	}
}

// ============================================================================
// Authentication Methods
// ============================================================================

// GetAuthStatus returns the current authentication status
func (a *App) GetAuthStatus() map[string]interface{} {
	return a.authService.GetAuthStatus()
}

// LoginWithGcloud initiates login using gcloud CLI
func (a *App) LoginWithGcloud() error {
	if err := a.authService.LoginWithGcloud(a.ctx); err != nil {
		return err
	}

	// Initialize authenticated services after successful login
	a.initializeAuthenticatedServices()

	return nil
}

// Logout logs out the current user
func (a *App) Logout() error {
	if err := a.authService.Logout(); err != nil {
		return err
	}

	// Clear authenticated services
	a.projectService = nil
	a.instanceService = nil
	if a.tunnelManager != nil {
		a.tunnelManager.StopAllTunnels()
	}
	a.tunnelManager = nil
	a.passwordService = nil

	return nil
}

// ============================================================================
// Project Methods
// ============================================================================

// ListProjects returns all accessible GCP projects
func (a *App) ListProjects() ([]gcp.Project, error) {
	if a.projectService == nil {
		a.initializeAuthenticatedServices()
	}
	if a.projectService == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	return a.projectService.ListProjects(a.ctx)
}

// ============================================================================
// Instance Methods
// ============================================================================

// ListInstances returns all instances in a project
func (a *App) ListInstances(projectID string) ([]gcp.Instance, error) {
	if a.instanceService == nil {
		a.initializeAuthenticatedServices()
	}
	if a.instanceService == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	return a.instanceService.ListInstances(a.ctx, projectID)
}

// ListWindowsInstances returns only Windows instances in a project
func (a *App) ListWindowsInstances(projectID string) ([]gcp.Instance, error) {
	if a.instanceService == nil {
		a.initializeAuthenticatedServices()
	}
	if a.instanceService == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	return a.instanceService.ListWindowsInstances(a.ctx, projectID)
}

// ============================================================================
// Tunnel Methods
// ============================================================================

// StartTunnel starts an IAP tunnel to the specified instance
func (a *App) StartTunnel(project, zone, instance string) (map[string]interface{}, error) {
	if a.tunnelManager == nil {
		a.initializeAuthenticatedServices()
	}
	if a.tunnelManager == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	t, err := a.tunnelManager.StartTunnel(a.ctx, project, zone, instance, 3389)
	if err != nil {
		return nil, err
	}

	return t.TunnelInfo(), nil
}

// StopTunnel stops an active tunnel
func (a *App) StopTunnel(tunnelID string) error {
	if a.tunnelManager == nil {
		return fmt.Errorf("not authenticated")
	}

	return a.tunnelManager.StopTunnel(tunnelID)
}

// StopAllTunnels stops all active tunnels
func (a *App) StopAllTunnels() {
	if a.tunnelManager != nil {
		a.tunnelManager.StopAllTunnels()
	}
}

// ListTunnels returns all active tunnels
func (a *App) ListTunnels() []map[string]interface{} {
	if a.tunnelManager == nil {
		return []map[string]interface{}{}
	}

	tunnels := a.tunnelManager.ListTunnels()
	result := make([]map[string]interface{}, len(tunnels))
	for i, t := range tunnels {
		result[i] = t.TunnelInfo()
	}
	return result
}

// ============================================================================
// Connection Methods
// ============================================================================

// GetConnections returns all saved connections
func (a *App) GetConnections() []config.Connection {
	if a.configService == nil {
		return []config.Connection{}
	}
	return a.configService.GetConnections()
}

// AddConnection adds a new connection
func (a *App) AddConnection(conn config.Connection) error {
	if a.configService == nil {
		return fmt.Errorf("config service not initialized")
	}
	return a.configService.AddConnection(conn)
}

// UpdateConnection updates an existing connection
func (a *App) UpdateConnection(conn config.Connection) error {
	if a.configService == nil {
		return fmt.Errorf("config service not initialized")
	}
	return a.configService.UpdateConnection(conn)
}

// DeleteConnection deletes a connection
func (a *App) DeleteConnection(id string) error {
	if a.configService == nil {
		return fmt.Errorf("config service not initialized")
	}
	return a.configService.DeleteConnection(id)
}

// Connect starts a tunnel and launches RDP for a connection
func (a *App) Connect(conn config.Connection) error {
	// Start tunnel if not already running
	existingTunnel := a.tunnelManager.GetTunnelForInstance(conn.Project, conn.Zone, conn.Instance, 3389)
	var localPort int

	if existingTunnel != nil && existingTunnel.Status == "running" {
		localPort = existingTunnel.LocalPort
	} else {
		tunnelInfo, err := a.StartTunnel(conn.Project, conn.Zone, conn.Instance)
		if err != nil {
			return fmt.Errorf("failed to start tunnel: %w", err)
		}
		localPort = int(tunnelInfo["localPort"].(int))
	}

	// Update last used time
	if a.configService != nil {
		a.configService.UpdateLastUsed(conn.ID)
	}

	// Launch RDP
	rdpConfig := rdp.DefaultConfig()
	rdpConfig.Port = localPort
	rdpConfig.Username = conn.Username
	rdpConfig.Domain = conn.Domain
	rdpConfig.ConnectionName = conn.Name

	if conn.RDPSettings.ScreenWidth > 0 {
		rdpConfig.ScreenWidth = conn.RDPSettings.ScreenWidth
	}
	if conn.RDPSettings.ScreenHeight > 0 {
		rdpConfig.ScreenHeight = conn.RDPSettings.ScreenHeight
	}
	rdpConfig.FullScreen = conn.RDPSettings.FullScreen
	rdpConfig.ClipboardShare = conn.RDPSettings.ClipboardShare
	rdpConfig.DriveRedirect = conn.RDPSettings.DriveRedirect
	rdpConfig.AudioMode = conn.RDPSettings.AudioMode

	return a.rdpService.LaunchRDP(rdpConfig)
}

// ============================================================================
// Keychain Methods
// ============================================================================

// StorePassword stores a password in the keychain
func (a *App) StorePassword(instance, username, password string) error {
	return a.keychainService.StorePassword(instance, username, password)
}

// GetPassword retrieves a password from the keychain
func (a *App) GetPassword(instance, username string) (string, error) {
	return a.keychainService.GetPassword(instance, username)
}

// HasPassword checks if a password exists in the keychain
func (a *App) HasPassword(instance, username string) bool {
	return a.keychainService.HasPassword(instance, username)
}

// ============================================================================
// Password Reset Methods
// ============================================================================

// ResetWindowsPassword resets a Windows user password
func (a *App) ResetWindowsPassword(project, zone, instance, username string) (string, error) {
	if a.passwordService == nil {
		a.initializeAuthenticatedServices()
	}
	if a.passwordService == nil {
		return "", fmt.Errorf("not authenticated")
	}

	return a.passwordService.ResetWindowsPassword(a.ctx, project, zone, instance, username)
}

// ============================================================================
// RDP Methods
// ============================================================================

// IsRDPClientInstalled checks if Microsoft Remote Desktop is installed
func (a *App) IsRDPClientInstalled() bool {
	return a.rdpService.IsMicrosoftRemoteDesktopInstalled()
}

// LaunchRDP launches RDP to the specified port
func (a *App) LaunchRDP(port int, username, instanceName string) error {
	return a.rdpService.LaunchRDPWithPort(port, username, instanceName)
}

// ============================================================================
// Settings Methods
// ============================================================================

// GetSettings returns the application settings
func (a *App) GetSettings() config.AppSettings {
	if a.configService == nil {
		return config.AppSettings{}
	}
	return a.configService.GetSettings()
}

// UpdateSettings updates the application settings
func (a *App) UpdateSettings(settings config.AppSettings) error {
	if a.configService == nil {
		return fmt.Errorf("config service not initialized")
	}
	return a.configService.UpdateSettings(settings)
}

// ============================================================================
// Utility Methods
// ============================================================================

// ShowMessage displays a message dialog
func (a *App) ShowMessage(title, message string) {
	runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.InfoDialog,
		Title:   title,
		Message: message,
	})
}

// ShowError displays an error dialog
func (a *App) ShowError(title, message string) {
	runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.ErrorDialog,
		Title:   title,
		Message: message,
	})
}

// ============================================================================
// Embedded RDP Viewer Methods
// ============================================================================

// StartViewerSession starts an embedded RDP viewer session with a specific ID
func (a *App) StartViewerSession(sessionId, tunnelID, username, password string, width, height int) error {
	if a.viewerService == nil {
		return fmt.Errorf("viewer service not initialized")
	}

	if a.tunnelManager == nil {
		return fmt.Errorf("not authenticated")
	}

	tunnel, err := a.tunnelManager.GetTunnel(tunnelID)
	if err != nil {
		return fmt.Errorf("tunnel not found: %w", err)
	}

	// Use provided resolution or defaults
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 800
	}

	return a.viewerService.ConnectSessionWithResolution(sessionId, "localhost", tunnel.LocalPort, username, password, width, height)
}

// StopViewerSession stops a specific viewer session
func (a *App) StopViewerSession(sessionId string) error {
	if a.viewerService == nil {
		return fmt.Errorf("viewer service not initialized")
	}

	a.viewerService.DisconnectSession(sessionId)
	return nil
}

// StopAllViewerSessions stops all viewer sessions
func (a *App) StopAllViewerSessions() error {
	if a.viewerService == nil {
		return fmt.Errorf("viewer service not initialized")
	}

	a.viewerService.DisconnectAll()
	return nil
}

// ViewerSessionMouseMove forwards mouse movement to a specific session
func (a *App) ViewerSessionMouseMove(sessionId string, x, y int) {
	if a.viewerService != nil {
		a.viewerService.SendMouseMove(sessionId, x, y)
	}
}

// ViewerSessionMouseClick forwards mouse click to a specific session
func (a *App) ViewerSessionMouseClick(sessionId string, x, y, button int, pressed bool) {
	if a.viewerService != nil {
		a.viewerService.SendMouseClick(sessionId, x, y, button, pressed)
	}
}

// ViewerSessionMouseWheel forwards mouse wheel to a specific session
func (a *App) ViewerSessionMouseWheel(sessionId string, scroll int) {
	if a.viewerService != nil {
		a.viewerService.SendMouseWheel(sessionId, scroll)
	}
}

// ViewerSessionKeyboard forwards keyboard input to a specific session
func (a *App) ViewerSessionKeyboard(sessionId string, keyCode int, pressed bool) {
	if a.viewerService != nil {
		a.viewerService.SendKeyboard(sessionId, keyCode, pressed)
	}
}

// IsViewerSessionConnected returns whether a specific session is connected
func (a *App) IsViewerSessionConnected(sessionId string) bool {
	if a.viewerService == nil {
		return false
	}
	return a.viewerService.IsSessionConnected(sessionId)
}

// GetViewerSessionIds returns all active session IDs
func (a *App) GetViewerSessionIds() []string {
	if a.viewerService == nil {
		return []string{}
	}
	return a.viewerService.GetSessionIds()
}

// ============================================================================
// Backward compatibility methods (use "default" session)
// ============================================================================

// StartViewer starts the embedded RDP viewer for a tunnel (backward compatible)
func (a *App) StartViewer(tunnelID, username, password string) error {
	return a.StartViewerSession("default", tunnelID, username, password, 1280, 800)
}

// StartViewerDirect starts the embedded RDP viewer connecting directly (backward compatible)
func (a *App) StartViewerDirect(host string, port int, username, password string) error {
	if a.viewerService == nil {
		return fmt.Errorf("viewer service not initialized")
	}
	return a.viewerService.ConnectSession("default", host, port, username, password)
}

// StopViewer stops the embedded RDP viewer (backward compatible)
func (a *App) StopViewer() error {
	return a.StopViewerSession("default")
}

// ViewerMouseMove forwards mouse movement (backward compatible - uses default session)
func (a *App) ViewerMouseMove(x, y int) {
	a.ViewerSessionMouseMove("default", x, y)
}

// ViewerMouseClick forwards mouse click (backward compatible - uses default session)
func (a *App) ViewerMouseClick(x, y, button int, pressed bool) {
	a.ViewerSessionMouseClick("default", x, y, button, pressed)
}

// ViewerMouseWheel forwards mouse wheel (backward compatible - uses default session)
func (a *App) ViewerMouseWheel(scroll int) {
	a.ViewerSessionMouseWheel("default", scroll)
}

// ViewerKeyboard forwards keyboard input (backward compatible - uses default session)
func (a *App) ViewerKeyboard(keyCode int, pressed bool) {
	a.ViewerSessionKeyboard("default", keyCode, pressed)
}

// IsViewerConnected returns whether the default session is connected
func (a *App) IsViewerConnected() bool {
	return a.IsViewerSessionConnected("default")
}

// SetViewerQuality sets the JPEG quality for the viewer
func (a *App) SetViewerQuality(quality int) {
	if a.viewerService != nil {
		a.viewerService.SetQuality(quality)
	}
}

// SetViewerResolution sets the resolution for the viewer
func (a *App) SetViewerResolution(width, height int) {
	if a.viewerService != nil {
		a.viewerService.SetResolution(width, height)
	}
}
