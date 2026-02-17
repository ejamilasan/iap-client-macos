package rdp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RDPService handles RDP connection launching
type RDPService struct {
	tempDir string
}

// RDPConfig contains RDP connection settings
type RDPConfig struct {
	Host           string
	Port           int
	Username       string
	Domain         string
	FullScreen     bool
	ScreenWidth    int
	ScreenHeight   int
	ColorDepth     int
	AudioMode      int    // 0=local, 1=remote, 2=none
	DriveRedirect  bool
	ClipboardShare bool
	GatewayUsage   int    // 0=none, 1=always, 2=detect
	ConnectionName string // Friendly name for the connection
}

// NewRDPService creates a new RDP service
func NewRDPService() *RDPService {
	// Use Application Support for persistent RDP files
	// This allows Microsoft Remote Desktop to remember credentials per connection
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.TempDir()
	}

	rdpDir := filepath.Join(homeDir, "Library", "Application Support", "IAP Client", "connections")
	os.MkdirAll(rdpDir, 0700)

	return &RDPService{
		tempDir: rdpDir,
	}
}

// DefaultConfig returns default RDP configuration
func DefaultConfig() RDPConfig {
	return RDPConfig{
		Port:           3389,
		FullScreen:     false,
		ScreenWidth:    1920,
		ScreenHeight:   1080,
		ColorDepth:     32,
		AudioMode:      0,
		DriveRedirect:  false,
		ClipboardShare: true,
		GatewayUsage:   0,
	}
}

// GenerateRDPFile creates an .rdp file with the given configuration
func (r *RDPService) GenerateRDPFile(config RDPConfig) (string, error) {
	var builder strings.Builder

	// Basic connection settings
	builder.WriteString(fmt.Sprintf("full address:s:localhost:%d\n", config.Port))

	if config.Username != "" {
		builder.WriteString(fmt.Sprintf("username:s:%s\n", config.Username))
	}

	if config.Domain != "" {
		builder.WriteString(fmt.Sprintf("domain:s:%s\n", config.Domain))
	}

	// Screen settings
	if config.FullScreen {
		builder.WriteString("screen mode id:i:2\n")
	} else {
		builder.WriteString("screen mode id:i:1\n")
		builder.WriteString(fmt.Sprintf("desktopwidth:i:%d\n", config.ScreenWidth))
		builder.WriteString(fmt.Sprintf("desktopheight:i:%d\n", config.ScreenHeight))
	}

	// Color depth
	builder.WriteString(fmt.Sprintf("session bpp:i:%d\n", config.ColorDepth))

	// Audio settings
	builder.WriteString(fmt.Sprintf("audiomode:i:%d\n", config.AudioMode))

	// Redirection settings
	if config.DriveRedirect {
		builder.WriteString("drivestoredirect:s:*\n")
	}

	if config.ClipboardShare {
		builder.WriteString("redirectclipboard:i:1\n")
	} else {
		builder.WriteString("redirectclipboard:i:0\n")
	}

	// Security settings
	builder.WriteString("authentication level:i:0\n")
	builder.WriteString("prompt for credentials:i:0\n") // Don't prompt if credentials saved in MS Remote Desktop
	builder.WriteString("negotiate security layer:i:1\n")
	builder.WriteString("enablecredsspsupport:i:1\n")

	// Performance settings for better experience over tunnel
	builder.WriteString("connection type:i:7\n")  // LAN
	builder.WriteString("networkautodetect:i:1\n")
	builder.WriteString("bandwidthautodetect:i:1\n")

	// Disable unnecessary features for better performance
	builder.WriteString("disable wallpaper:i:0\n")
	builder.WriteString("disable full window drag:i:0\n")
	builder.WriteString("disable menu anims:i:1\n")
	builder.WriteString("disable themes:i:0\n")
	builder.WriteString("disable cursor setting:i:0\n")
	builder.WriteString("bitmapcachepersistenable:i:1\n")

	// Gateway settings
	builder.WriteString(fmt.Sprintf("gatewayusagemethod:i:%d\n", config.GatewayUsage))

	// Create the RDP file
	fileName := "connection.rdp"
	if config.ConnectionName != "" {
		// Sanitize the connection name for use as filename
		safeName := strings.ReplaceAll(config.ConnectionName, "/", "-")
		safeName = strings.ReplaceAll(safeName, "\\", "-")
		safeName = strings.ReplaceAll(safeName, ":", "-")
		fileName = fmt.Sprintf("%s.rdp", safeName)
	}

	filePath := filepath.Join(r.tempDir, fileName)
	err := os.WriteFile(filePath, []byte(builder.String()), 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write RDP file: %w", err)
	}

	return filePath, nil
}

// LaunchRDP launches Microsoft Remote Desktop with the given configuration
func (r *RDPService) LaunchRDP(config RDPConfig) error {
	filePath, err := r.GenerateRDPFile(config)
	if err != nil {
		return err
	}

	// Open the RDP file with the default application (Microsoft Remote Desktop)
	cmd := exec.Command("open", filePath)
	return cmd.Run()
}

// LaunchRDPWithPort launches RDP to localhost on the specified port
func (r *RDPService) LaunchRDPWithPort(port int, username, instanceName string) error {
	config := DefaultConfig()
	config.Port = port
	config.Username = username
	config.ConnectionName = instanceName

	return r.LaunchRDP(config)
}

// IsMicrosoftRemoteDesktopInstalled checks if Microsoft Remote Desktop is installed
func (r *RDPService) IsMicrosoftRemoteDesktopInstalled() bool {
	// Check common installation paths for Microsoft Remote Desktop
	paths := []string{
		"/Applications/Microsoft Remote Desktop.app",
		"/Applications/Microsoft Remote Desktop Beta.app",
		"/Applications/Windows App.app", // New name for Microsoft Remote Desktop
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// CleanupTempFiles removes temporary RDP files
func (r *RDPService) CleanupTempFiles() error {
	files, err := filepath.Glob(filepath.Join(r.tempDir, "*.rdp"))
	if err != nil {
		return err
	}

	for _, file := range files {
		os.Remove(file)
	}

	return nil
}
