package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	App                     AppConfig      `json:"app"`
	Log                     LogConfig      `json:"log"`
	Database                DatabaseConfig `json:"database"`
	MQTT                    MQTTConfig     `json:"mqtt"`
	Ollama                  OllamaConfig   `json:"ollama"`
	WebDAV                  WebDAVConfig   `json:"webdav"`
	AssociationThreshold    float64        `json:"association_threshold"`
	BriefRelevanceThreshold float64        `json:"brief_relevance_threshold"`
}

type WebDAVConfig struct {
	BaseURL      string `json:"base_url"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	StandingPath string `json:"standing_path"`
	EntriesPath  string `json:"entries_path"`
}

type AppConfig struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Env     string `json:"env"`
}

type LogConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	SSLMode  string `json:"sslmode"`
}

// ConnectionString returns a Postgres connection string built from individual fields.
func (d DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode)
}

type MQTTConfig struct {
	BrokerURL string `json:"broker_url"`
	ClientID  string `json:"client_id"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

type OllamaConfig struct {
	BaseURL    string `json:"base_url"`
	EmbedModel string `json:"embed_model"`
	ChatModel  string `json:"chat_model"`
	ChatNumCtx int    `json:"chat_num_ctx"`
	Timeout    int    `json:"timeout"`
}

// Load configuration from environment variables.
func Load(configPath string) (*Config, error) {
	if configPath != "" {
		if err := godotenv.Load(configPath); err != nil {
			return nil, fmt.Errorf("failed to load env file %s: %w", configPath, err)
		}
	} else {
		godotenv.Load()
	}

	config := &Config{
		App: AppConfig{
			Name:    getEnv("APP_NAME", "journal"),
			Version: getEnv("APP_VERSION", "0.1.0"),
			Env:     getEnv("APP_ENV", "development"),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "journal"),
			Password: getEnv("DB_PASSWORD", "journal"),
			DBName:   getEnv("DB_NAME", "journal"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		MQTT: MQTTConfig{
			BrokerURL: getEnv("MQTT_BROKER_URL", "tcp://localhost:1883"),
			ClientID:  getEnv("MQTT_CLIENT_ID", "journal"),
			Username:  getEnv("MQTT_USER", ""),
			Password:  getEnv("MQTT_PASSWORD", ""),
		},
		Ollama: OllamaConfig{
			BaseURL:    getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
			EmbedModel: getEnv("OLLAMA_EMBED_MODEL", "qwen3-embedding:8b"),
			ChatModel:  getEnv("OLLAMA_CHAT_MODEL", "qwen2.5:7b"),
			ChatNumCtx: getEnvInt("OLLAMA_CHAT_NUM_CTX", 32768),
			Timeout:    getEnvInt("OLLAMA_TIMEOUT", 60),
		},
		WebDAV: WebDAVConfig{
			BaseURL:      getEnv("WEBDAV_URL", ""),
			Username:     getEnv("WEBDAV_USERNAME", ""),
			Password:     getEnv("WEBDAV_PASSWORD", ""),
			StandingPath: getEnv("WEBDAV_STANDING_PATH", "/standing/"),
			EntriesPath:  getEnv("WEBDAV_ENTRIES_PATH", "/entries/"),
		},
	}

	config.AssociationThreshold = getEnvFloat("ASSOCIATION_THRESHOLD", 0.3)
	config.BriefRelevanceThreshold = getEnvFloat("BRIEF_RELEVANCE_THRESHOLD", 0.6)

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := parseInt(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := parseFloat(value); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
