package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Connection represents a saved connection profile
type Connection struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Project      string    `json:"project"`
	Zone         string    `json:"zone"`
	Instance     string    `json:"instance"`
	Username     string    `json:"username"`
	Domain       string    `json:"domain,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	LastUsedAt   time.Time `json:"lastUsedAt,omitempty"`
	RDPSettings  RDPSettings `json:"rdpSettings,omitempty"`
}

// RDPSettings contains RDP-specific settings for a connection
type RDPSettings struct {
	FullScreen     bool `json:"fullScreen"`
	ScreenWidth    int  `json:"screenWidth"`
	ScreenHeight   int  `json:"screenHeight"`
	ColorDepth     int  `json:"colorDepth"`
	AudioMode      int  `json:"audioMode"`
	DriveRedirect  bool `json:"driveRedirect"`
	ClipboardShare bool `json:"clipboardShare"`
}

// Config represents the application configuration
type Config struct {
	Connections []Connection `json:"connections"`
	Settings    AppSettings  `json:"settings"`
}

// AppSettings contains application-wide settings
type AppSettings struct {
	DefaultProject    string `json:"defaultProject,omitempty"`
	AutoConnect       bool   `json:"autoConnect"`
	ShowWindowsOnly   bool   `json:"showWindowsOnly"`
	RememberPasswords bool   `json:"rememberPasswords"`
}

// ConfigService handles configuration persistence
type ConfigService struct {
	configPath string
	config     *Config
	mu         sync.RWMutex
}

// NewConfigService creates a new configuration service
func NewConfigService() (*ConfigService, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "iap-client-macos")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.json")

	cs := &ConfigService{
		configPath: configPath,
		config: &Config{
			Connections: []Connection{},
			Settings: AppSettings{
				ShowWindowsOnly:   true,
				RememberPasswords: true,
			},
		},
	}

	// Load existing config if it exists
	if err := cs.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return cs, nil
}

// load reads the configuration from disk
func (cs *ConfigService) load() error {
	data, err := os.ReadFile(cs.configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, cs.config)
}

// save writes the configuration to disk
func (cs *ConfigService) save() error {
	data, err := json.MarshalIndent(cs.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cs.configPath, data, 0600)
}

// GetConnections returns all saved connections
func (cs *ConfigService) GetConnections() []Connection {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	connections := make([]Connection, len(cs.config.Connections))
	copy(connections, cs.config.Connections)
	return connections
}

// GetConnection returns a specific connection by ID
func (cs *ConfigService) GetConnection(id string) (*Connection, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, conn := range cs.config.Connections {
		if conn.ID == id {
			return &conn, nil
		}
	}

	return nil, fmt.Errorf("connection not found: %s", id)
}

// AddConnection adds a new connection (replaces existing if same VM)
func (cs *ConfigService) AddConnection(conn Connection) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Check for existing connection (same project, zone, instance) and replace it
	for i, existing := range cs.config.Connections {
		if existing.Project == conn.Project &&
			existing.Zone == conn.Zone &&
			existing.Instance == conn.Instance {
			// Replace existing connection, preserving its ID and creation time
			conn.ID = existing.ID
			conn.CreatedAt = existing.CreatedAt
			conn.UpdatedAt = time.Now()
			cs.config.Connections[i] = conn
			return cs.save()
		}
	}

	// Generate ID if not provided
	if conn.ID == "" {
		conn.ID = fmt.Sprintf("%s-%s-%s-%d", conn.Project, conn.Zone, conn.Instance, time.Now().UnixNano())
	}

	conn.CreatedAt = time.Now()
	conn.UpdatedAt = time.Now()

	// Set default RDP settings
	if conn.RDPSettings.ScreenWidth == 0 {
		conn.RDPSettings.ScreenWidth = 1920
	}
	if conn.RDPSettings.ScreenHeight == 0 {
		conn.RDPSettings.ScreenHeight = 1080
	}
	if conn.RDPSettings.ColorDepth == 0 {
		conn.RDPSettings.ColorDepth = 32
	}
	conn.RDPSettings.ClipboardShare = true

	cs.config.Connections = append(cs.config.Connections, conn)
	return cs.save()
}

// UpdateConnection updates an existing connection
func (cs *ConfigService) UpdateConnection(conn Connection) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i, c := range cs.config.Connections {
		if c.ID == conn.ID {
			conn.CreatedAt = c.CreatedAt
			conn.UpdatedAt = time.Now()
			cs.config.Connections[i] = conn
			return cs.save()
		}
	}

	return fmt.Errorf("connection not found: %s", conn.ID)
}

// DeleteConnection removes a connection
func (cs *ConfigService) DeleteConnection(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i, conn := range cs.config.Connections {
		if conn.ID == id {
			cs.config.Connections = append(cs.config.Connections[:i], cs.config.Connections[i+1:]...)
			return cs.save()
		}
	}

	return fmt.Errorf("connection not found: %s", id)
}

// UpdateLastUsed updates the last used timestamp for a connection
func (cs *ConfigService) UpdateLastUsed(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i, conn := range cs.config.Connections {
		if conn.ID == id {
			cs.config.Connections[i].LastUsedAt = time.Now()
			return cs.save()
		}
	}

	return nil
}

// GetSettings returns the application settings
func (cs *ConfigService) GetSettings() AppSettings {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.config.Settings
}

// UpdateSettings updates the application settings
func (cs *ConfigService) UpdateSettings(settings AppSettings) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.config.Settings = settings
	return cs.save()
}

// FindConnectionByInstance finds a connection by project/zone/instance
func (cs *ConfigService) FindConnectionByInstance(project, zone, instance string) *Connection {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, conn := range cs.config.Connections {
		if conn.Project == project && conn.Zone == zone && conn.Instance == instance {
			return &conn
		}
	}

	return nil
}
