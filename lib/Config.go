package lib

import (
  "os"
  "path/filepath"
  "encoding/json"
  
)

// Config represents the configuration from ~/config.json
type Config struct {
	Rules struct {
		RTT          time.Duration `json:"rtt"`
		MaxRetryCon  int           `json:"max-retry-con"`
		EnableRTT    bool          `json:"enable_rtt"`
	} `json:"rules"`
}



// loadConfig loads configuration from ~/config.json
func loadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	
	configPath := filepath.Join(home, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	
	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	
	return &config, nil
}
