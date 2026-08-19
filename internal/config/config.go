package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config aggregates all operational parameters required to boot and run the store_order microservice.
type Config struct {
	Port                 string
	Dev                  bool
	EnableDocs           bool
	DatabaseURL          string
	KafkaBrokers         []string
	ProductServiceURL    string
	MidtransServerKey    string
	MidtransClientKey    string
	MidtransIsProduction bool
}

// Load loads environment variables from .env if present and constructs a validated Config instance.
// Why: Centralizes environment parsing and validation at application bootstrap to ensure fast-fail configuration integrity.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("[Config] No .env file found or failed to load, falling back to system environment variables.")
	}

	port := getEnv("PORT", "8060")
	devStr := strings.ToLower(strings.TrimSpace(getEnv("DEV", "false")))
	dev := devStr == "true" || devStr == "1"

	enableDocsStr := strings.ToLower(strings.TrimSpace(getEnv("ENABLE_DOCS", "true")))
	enableDocs := enableDocsStr == "true" || enableDocsStr == "1"

	databaseURL := getEnv("DATABASE_URL", "")
	
	kafkaBrokersStr := getEnv("KAFKA_BROKERS", "localhost:9092")
	kafkaBrokers := splitAndTrim(kafkaBrokersStr, ",")

	productServiceURL := strings.TrimRight(getEnv("PRODUCT_SERVICE_URL", "http://localhost:8040"), "/")

	midtransServerKey := getEnv("MIDTRANS_SERVER_KEY", "")
	midtransClientKey := getEnv("MIDTRANS_CLIENT_KEY", "")
	midtransIsProd, _ := strconv.ParseBool(getEnv("MIDTRANS_IS_PRODUCTION", "false"))

	return &Config{
		Port:                 port,
		Dev:                  dev,
		EnableDocs:           enableDocs,
		DatabaseURL:          databaseURL,
		KafkaBrokers:         kafkaBrokers,
		ProductServiceURL:    productServiceURL,
		MidtransServerKey:    midtransServerKey,
		MidtransClientKey:    midtransClientKey,
		MidtransIsProduction: midtransIsProd,
	}
}

// getEnv retrieves an environment variable or returns the default value if unset.
func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && strings.TrimSpace(val) != "" {
		return val
	}
	return defaultVal
}

// splitAndTrim splits a delimited string and trims whitespace around each element.
func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	var res []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	if len(res) == 0 {
		return []string{"localhost:9092"}
	}
	return res
}
