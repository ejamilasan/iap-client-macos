package rdp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// PasswordService handles Windows password reset via GCP API
type PasswordService struct {
	tokenSource oauth2.TokenSource
}

// PasswordResetResponse contains the result of a password reset
type PasswordResetResponse struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Encrypted bool   `json:"encrypted"`
}

// serialPortResponse is the JSON structure from serial port output
type serialPortResponse struct {
	Modulus           string `json:"modulus"`
	Exponent          string `json:"exponent"`
	UserName          string `json:"userName"`
	EncryptedPassword string `json:"encryptedPassword"`
	ErrorMessage      string `json:"errorMessage"`
}

// NewPasswordService creates a new password service
func NewPasswordService(ts oauth2.TokenSource) *PasswordService {
	return &PasswordService{
		tokenSource: ts,
	}
}

// SetTokenSource updates the token source
func (ps *PasswordService) SetTokenSource(ts oauth2.TokenSource) {
	ps.tokenSource = ts
}

// ResetWindowsPassword resets the password for a Windows user on a GCE instance
// Returns the new password
func (ps *PasswordService) ResetWindowsPassword(ctx context.Context, project, zone, instance, username string) (string, error) {
	if ps.tokenSource == nil {
		return "", fmt.Errorf("not authenticated")
	}

	// Generate an RSA key pair for secure password transmission
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Encode modulus and exponent for the metadata
	modulus := base64.StdEncoding.EncodeToString(privateKey.N.Bytes())

	// Exponent needs to be encoded as big-endian bytes
	expBytes := big.NewInt(int64(privateKey.E)).Bytes()
	exponent := base64.StdEncoding.EncodeToString(expBytes)

	// Create Compute Engine service
	computeService, err := compute.NewService(ctx, option.WithTokenSource(ps.tokenSource))
	if err != nil {
		return "", fmt.Errorf("failed to create compute service: %w", err)
	}

	// Create the metadata value for password reset
	expireTime := time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339)
	metadataMap := map[string]string{
		"userName": username,
		"modulus":  modulus,
		"exponent": exponent,
		"expireOn": expireTime,
	}
	metadataBytes, err := json.Marshal(metadataMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	metadataValue := string(metadataBytes)

	// Get the current instance metadata
	inst, err := computeService.Instances.Get(project, zone, instance).Do()
	if err != nil {
		return "", fmt.Errorf("failed to get instance: %w", err)
	}

	// Find or create the windows-keys metadata
	var existingFingerprint string
	var items []*compute.MetadataItems
	if inst.Metadata != nil {
		existingFingerprint = inst.Metadata.Fingerprint
		items = inst.Metadata.Items
	}

	// Add or update the windows-keys metadata
	found := false
	for _, item := range items {
		if item.Key == "windows-keys" {
			item.Value = &metadataValue
			found = true
			break
		}
	}

	if !found {
		items = append(items, &compute.MetadataItems{
			Key:   "windows-keys",
			Value: &metadataValue,
		})
	}

	// Set the new metadata
	metadata := &compute.Metadata{
		Fingerprint: existingFingerprint,
		Items:       items,
	}

	op, err := computeService.Instances.SetMetadata(project, zone, instance, metadata).Do()
	if err != nil {
		return "", fmt.Errorf("failed to set metadata: %w", err)
	}

	// Wait for the operation to complete
	err = waitForOperation(ctx, computeService, project, zone, op.Name)
	if err != nil {
		return "", fmt.Errorf("failed waiting for metadata operation: %w", err)
	}

	// Poll the serial port output for the encrypted password
	password, err := ps.waitForPassword(ctx, computeService, project, zone, instance, privateKey, modulus)
	if err != nil {
		return "", fmt.Errorf("failed to get password: %w", err)
	}

	return password, nil
}

// waitForPassword polls the serial port for the encrypted password and decrypts it
func (ps *PasswordService) waitForPassword(ctx context.Context, svc *compute.Service, project, zone, instance string, privateKey *rsa.PrivateKey, expectedModulus string) (string, error) {
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastStart int64 = 0

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for password (Windows agent may not be running)")
		case <-ticker.C:
			// Check serial port 4 for password response
			output, err := svc.Instances.GetSerialPortOutput(project, zone, instance).Port(4).Start(lastStart).Do()
			if err != nil {
				continue // Keep trying
			}

			if output.Next > lastStart {
				lastStart = output.Next
			}

			// Parse the serial output for password response
			password, found, err := parsePasswordFromOutput(output.Contents, privateKey, expectedModulus)
			if err != nil {
				return "", err
			}
			if found {
				return password, nil
			}
		}
	}
}

// parsePasswordFromOutput parses the serial port output for encrypted password
func parsePasswordFromOutput(output string, privateKey *rsa.PrivateKey, expectedModulus string) (string, bool, error) {
	// The output contains JSON objects, one per line
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var resp serialPortResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue // Not valid JSON, skip
		}

		// Check if this response matches our request (same modulus)
		if resp.Modulus != expectedModulus {
			continue
		}

		// Check for error message
		if resp.ErrorMessage != "" {
			return "", true, fmt.Errorf("password reset failed: %s", resp.ErrorMessage)
		}

		// Decrypt the password
		if resp.EncryptedPassword == "" {
			continue
		}

		encryptedBytes, err := base64.StdEncoding.DecodeString(resp.EncryptedPassword)
		if err != nil {
			return "", true, fmt.Errorf("failed to decode encrypted password: %w", err)
		}

		// Try OAEP with SHA-1 first (most common for Windows agent)
		decrypted, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, privateKey, encryptedBytes, nil)
		if err != nil {
			// Try SHA-256 as fallback (newer agents)
			decrypted, err = rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encryptedBytes, nil)
			if err != nil {
				// Try PKCS1v15 as last resort
				decrypted, err = rsa.DecryptPKCS1v15(rand.Reader, privateKey, encryptedBytes)
				if err != nil {
					return "", true, fmt.Errorf("failed to decrypt password: %w", err)
				}
			}
		}

		return string(decrypted), true, nil
	}

	return "", false, nil
}

// waitForOperation waits for a GCE operation to complete
func waitForOperation(ctx context.Context, svc *compute.Service, project, zone, opName string) error {
	for {
		op, err := svc.ZoneOperations.Get(project, zone, opName).Do()
		if err != nil {
			return err
		}

		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("operation failed: %s", op.Error.Errors[0].Message)
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			continue
		}
	}
}

// GeneratePassword generates a random password
func GeneratePassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	password := make([]byte, length)
	for i := range password {
		b := make([]byte, 1)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		password[i] = charset[int(b[0])%len(charset)]
	}
	return string(password), nil
}
