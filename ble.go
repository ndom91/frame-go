package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

const (
	// Custom service UUID for photo frame setup
	PhotoFrameServiceUUID = "c63dc91d-efa5-43f2-9e26-7d30ae8eb4fc"

	// Characteristic UUIDs
	WiFiCredentialsUUID = "12345678-1234-1234-1234-123456789001"
	FrameConfigUUID     = "12345678-1234-1234-1234-123456789002"
	SetupStatusUUID     = "12345678-1234-1234-1234-123456789003"
	CommandUUID         = "12345678-1234-1234-1234-123456789004"
)

type WiFiCredentials struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}

type FrameConfig struct {
	Name        string `json:"name"`
	FrameID     string `json:"frame_id"`
	APIEndpoint string `json:"api_endpoint"`
	S3Bucket    string `json:"s3_bucket"`
}

type SetupServer struct {
	adapter     *bluetooth.Adapter
	wifiCreds   *WiFiCredentials
	frameConfig *FrameConfig

	// Characteristics
	wifiChar   bluetooth.Characteristic
	configChar bluetooth.Characteristic
	statusChar bluetooth.Characteristic
	cmdChar    bluetooth.Characteristic

	// Status
	setupComplete bool
	statusMessage string

	// Generated frame ID
	frameID string

	// Configuration storage
	configData map[string]interface{}
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

// func NewSetupServer() *SetupServer {
// 	return &SetupServer{
// 		statusMessage: "Ready for setup",
// 	}
// }

func NewSetupServer() *SetupServer {
	server := &SetupServer{
		statusMessage: "Ready for setup",
		configData:    make(map[string]interface{}),
	}

	// Generate a random frame ID
	server.frameID = randomString(7)
	log.Printf("Generated Frame ID: %s", server.frameID)

	// Save the frame ID immediately
	server.saveConfigValue("frame_id", server.frameID)
	server.saveConfigValue("created_at", time.Now().Format(time.RFC3339))

	return server
}

func (s *SetupServer) Start() error {
	// Load existing configuration first
	if err := s.loadConfigFile(); err != nil {
		log.Printf("Warning: Could not load existing config: %v", err)
	}

	// Check if we already have a frame_id, if not generate one
	if _, exists := s.getConfigValue("frame_id"); !exists {
		s.saveConfigValue("frame_id", s.frameID)
	} else {
		// Use existing frame_id
		if existingID, exists := s.getConfigValue("frame_id"); exists {
			if idStr, ok := existingID.(string); ok {
				s.frameID = idStr
				log.Printf("Using existing Frame ID: %s", s.frameID)
			}
		}
	}

	s.adapter = bluetooth.DefaultAdapter

	err := s.adapter.Enable()
	if err != nil {
		return fmt.Errorf("failed to enable BLE adapter: %v", err)
	}
	// Parse UUIDs
	serviceUUID, err := bluetooth.ParseUUID(PhotoFrameServiceUUID)
	if err != nil {
		return fmt.Errorf("failed to parse service UUID: %v", err)
	}

	wifiUUID, err := bluetooth.ParseUUID(WiFiCredentialsUUID)
	if err != nil {
		return fmt.Errorf("failed to parse WiFi UUID: %v", err)
	}

	configUUID, err := bluetooth.ParseUUID(FrameConfigUUID)
	if err != nil {
		return fmt.Errorf("failed to parse config UUID: %v", err)
	}

	statusUUID, err := bluetooth.ParseUUID(SetupStatusUUID)
	if err != nil {
		return fmt.Errorf("failed to parse status UUID: %v", err)
	}

	cmdUUID, err := bluetooth.ParseUUID(CommandUUID)
	if err != nil {
		return fmt.Errorf("failed to parse command UUID: %v", err)
	}

	// Add the service
	err = s.adapter.AddService(&bluetooth.Service{
		UUID: serviceUUID,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &s.wifiChar,
				UUID:   wifiUUID,
				Flags:  bluetooth.CharacteristicWritePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					s.handleWiFiCredentials(value)
				},
			},
			{
				Handle: &s.configChar,
				UUID:   configUUID,
				Flags:  bluetooth.CharacteristicWritePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					s.handleFrameConfig(value)
				},
			},
			{
				Handle: &s.statusChar,
				UUID:   statusUUID,
				Flags:  bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicNotifyPermission,
				// ReadEvent: func(client bluetooth.Connection, offset int) ([]byte, error) {
				// 	return []byte(s.getCurrentStatus()), nil
				// },
			},
			{
				Handle: &s.cmdChar,
				UUID:   cmdUUID,
				Flags:  bluetooth.CharacteristicWritePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					s.handleCommand(value)
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add service: %v", err)
	}

	// Start advertising
	adv := s.adapter.DefaultAdvertisement()
	err = adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    "DominoFrame-" + s.frameID,
		ServiceUUIDs: []bluetooth.UUID{serviceUUID},
	})
	if err != nil {
		return fmt.Errorf("failed to configure advertisement: %v", err)
	}

	err = adv.Start()
	if err != nil {
		return fmt.Errorf("failed to start advertisement: %v", err)
	}

	// s.saveConfiguration()
	log.Println("BLE GATT server started, advertising as 'DominoFrame-'" + s.frameID)
	s.updateStatus("Ready for setup")

	return nil
}

func (s *SetupServer) handleWiFiCredentials(data []byte) {
	log.Printf("Received WiFi credentials: %d bytes", len(data))

	var creds WiFiCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		log.Printf("Failed to parse WiFi credentials: %v", err)
		s.updateStatus("Error: Invalid WiFi data")
		return
	}

	s.wifiCreds = &creds
	log.Printf("WiFi credentials received - SSID: %s", creds.SSID)
	s.updateStatus("WiFi credentials received")
	s.notifyStatusUpdate()
}

func (s *SetupServer) handleFrameConfig(data []byte) {
	log.Printf("Received frame config: %d bytes", len(data))

	var config FrameConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Failed to parse frame config: %v", err)
		s.updateStatus("Error: Invalid config data")
		s.notifyStatusUpdate()
		return
	}

	s.frameConfig = &config
	log.Printf("Frame config received - Name: %s, ID: %s", config.Name, config.FrameID)
	s.updateStatus("Frame config received")
	s.notifyStatusUpdate()
}

func (s *SetupServer) handleCommand(data []byte) {
	command := string(data)
	log.Printf("Received command: %s", command)

	switch command {
	case "complete_setup":
		s.completeSetup()
	case "reset":
		s.resetSetup()
	case "test_wifi":
		s.testWiFiConnection()
	default:
		log.Printf("Unknown command: %s", command)
		s.updateStatus("Error: Unknown command")
		s.notifyStatusUpdate()
	}
}

func (s *SetupServer) completeSetup() {
	log.Printf("Starting setup completion. WiFi creds: %v, Frame config: %v",
		s.wifiCreds != nil, s.frameConfig != nil)

	if s.wifiCreds == nil {
		log.Println("WiFi credentials are missing")
		s.updateStatus("Error: WiFi credentials missing")
		s.notifyStatusUpdate()
		return
	}

	if s.frameConfig == nil {
		log.Println("Frame configuration is missing")
		s.updateStatus("Error: Frame configuration missing")
		s.notifyStatusUpdate()
		return
	}

	s.updateStatus("Connecting to WiFi...")
	s.notifyStatusUpdate()

	// Connect to WiFi
	if err := s.connectToWiFi(); err != nil {
		log.Printf("Failed to connect to WiFi: %v", err)
		s.updateStatus("Error: WiFi connection failed")
		s.notifyStatusUpdate()
		return
	}

	s.updateStatus("WiFi connected, saving config...")
	s.notifyStatusUpdate()

	// Save configuration
	if err := s.saveConfiguration(); err != nil {
		log.Printf("Failed to save configuration: %v", err)
		s.updateStatus("Error: Failed to save config")
		s.notifyStatusUpdate()
		return
	}

	s.setupComplete = true
	s.updateStatus("Setup complete! Restarting...")
	s.notifyStatusUpdate()

	// Give time for the status to be read, then restart the main app
	go func() {
		time.Sleep(2 * time.Second)
		log.Println("Setup complete, would restart main photo frame app here")
		// You might want to signal your main app or restart the service
	}()
}

func (s *SetupServer) connectToWiFi() error {
	if s.wifiCreds == nil {
		return fmt.Errorf("no WiFi credentials provided")
	}

	// Use NetworkManager CLI (nmcli) - more reliable than wpa_cli
	// First, check if the network already exists
	cmd := exec.Command("nmcli", "connection", "show", s.wifiCreds.SSID)
	if err := cmd.Run(); err == nil {
		// Connection exists, just modify it
		cmd = exec.Command("nmcli", "connection", "modify", s.wifiCreds.SSID,
			"wifi-sec.psk", s.wifiCreds.Password)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to modify existing connection: %v", err)
		}
	} else {
		// Create new connection
		cmd = exec.Command("nmcli", "device", "wifi", "connect", s.wifiCreds.SSID,
			"password", s.wifiCreds.Password)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to connect to WiFi: %v", err)
		}
	}

	// Wait a moment for connection to establish
	time.Sleep(3 * time.Second)

	// Verify connection
	cmd = exec.Command("nmcli", "-t", "-f", "ACTIVE,SSID", "dev", "wifi")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to verify connection: %v", err)
	}

	connected := false
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) == 2 && parts[0] == "yes" && parts[1] == s.wifiCreds.SSID {
			connected = true
			break
		}
	}

	if !connected {
		return fmt.Errorf("connection verification failed")
	}

	return nil
}

// saveConfigValue saves a single key-value pair to the configuration
func (s *SetupServer) saveConfigValue(key string, value interface{}) error {
	s.configData[key] = value
	return s.writeConfigFile()
}

// saveConfigValues saves multiple key-value pairs to the configuration
func (s *SetupServer) saveConfigValues(values map[string]interface{}) error {
	for key, value := range values {
		s.configData[key] = value
	}
	return s.writeConfigFile()
}

// getConfigValue retrieves a value from the configuration
func (s *SetupServer) getConfigValue(key string) (interface{}, bool) {
	value, exists := s.configData[key]
	return value, exists
}

// writeConfigFile writes the current configuration to disk
func (s *SetupServer) writeConfigFile() error {
	data, err := json.MarshalIndent(s.configData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	err = os.WriteFile("./config.json", data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	log.Printf("Configuration saved to config.json")
	return nil
}

// loadConfigFile loads existing configuration from disk
func (s *SetupServer) loadConfigFile() error {
	data, err := os.ReadFile("./config.json")
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("No existing config file found, starting fresh")
			return nil
		}
		return fmt.Errorf("failed to read config file: %v", err)
	}

	err = json.Unmarshal(data, &s.configData)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}

	log.Printf("Loaded existing configuration with %d keys", len(s.configData))
	return nil
}

func (s *SetupServer) saveConfiguration() error {
	// Add defensive nil checks
	if s.frameConfig == nil {
		return fmt.Errorf("frame configuration is nil")
	}

	// Build configuration map with frame config + existing data
	configUpdates := map[string]interface{}{
		"frame_name":     s.frameConfig.Name,
		"api_endpoint":   s.frameConfig.APIEndpoint,
		"s3_bucket":      s.frameConfig.S3Bucket,
		"setup_complete": true,
		"setup_time":     time.Now().Format(time.RFC3339),
	}

	// Note: We keep the auto-generated frame_id, don't overwrite it
	// with the one from frameConfig unless it's specifically needed

	return s.saveConfigValues(configUpdates)
}

func (s *SetupServer) testWiFiConnection() {
	if s.wifiCreds == nil {
		s.updateStatus("Error: No WiFi credentials")
		s.notifyStatusUpdate()
		return
	}

	s.updateStatus("Testing WiFi connection...")
	s.notifyStatusUpdate()

	// Test internet connectivity
	cmd := exec.Command("ping", "-c", "1", "-W", "3", "1.1.1.1")
	if err := cmd.Run(); err != nil {
		s.updateStatus("WiFi test failed: No internet")
	} else {
		s.updateStatus("WiFi test successful")
	}
	s.notifyStatusUpdate()
}

func (s *SetupServer) resetSetup() {
	s.wifiCreds = nil
	s.frameConfig = nil
	s.setupComplete = false
	log.Println("Setup data reset")
	s.updateStatus("Setup reset, ready for new configuration")
	s.notifyStatusUpdate()
}

func (s *SetupServer) updateStatus(status string) {
	s.statusMessage = status
}

func (s *SetupServer) notifyStatusUpdate() {
	// Send notification to any connected clients
	// In the TinyGo Bluetooth library, notifications would be sent
	// to subscribed clients automatically when the characteristic is updated
	log.Printf("Status update: %s", s.statusMessage)
}

func (s *SetupServer) getCurrentStatus() string {
	if s.setupComplete {
		return "Setup complete"
	}

	status := "Ready for setup"
	if s.wifiCreds != nil {
		status += " | WiFi: ✓"
	}
	if s.frameConfig != nil {
		status += " | Config: ✓"
	}

	return status
}

func (s *SetupServer) Stop() {
	if s.adapter != nil {
		// Stop advertising and cleanup
		log.Println("Stopping BLE server...")
	}
}
