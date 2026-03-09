package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"nfc-tool/connector/internal/api"
	"nfc-tool/connector/internal/bridge"
)

var version = "dev"
var buildTime = "unknown"

const defaultAllowedOrigins = "http://localhost:*,https://localhost:*,http://127.0.0.1:*,https://127.0.0.1:*,https://nfc-tool.abcd854884.workers.dev,https://nfc-tool.abcd854884.workers.dev."

func main() {
	addr := getenv("NFC_CONNECTOR_ADDR", "127.0.0.1:42619")
	secret := getenv("NFC_CONNECTOR_SHARED_SECRET", "development-shared-secret")
	allowedOrigins := strings.Split(getenv("NFC_CONNECTOR_ALLOWED_ORIGINS", defaultAllowedOrigins), ",")
	driverMode := getenv("NFC_CONNECTOR_DRIVER", "auto")
	readerName := getenv("NFC_CONNECTOR_MOCK_READER", "Mock ACR1252U-M1")

	driver, err := buildDriver(driverMode, readerName)
	if err != nil {
		log.Fatalf("connector driver init: %v", err)
	}
	defer driver.Close()

	service := bridge.NewService(driver)
	server := api.NewServer(service, allowedOrigins, secret, version, buildTime)

	log.Printf("nfc connector listening on http://%s (driver=%s version=%s buildTime=%s)", addr, service.DriverName(), version, buildTime)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}

func buildDriver(mode string, readerName string) (bridge.Driver, error) {
	switch mode {
	case "mock":
		return bridge.NewMockDriver(readerName), nil
	case "pcsc":
		return bridge.NewPCSCDriver()
	default:
		pcscDriver, err := bridge.NewPCSCDriver()
		if err == nil {
			return pcscDriver, nil
		}
		log.Printf("pcsc unavailable, falling back to mock driver: %v", err)
		return bridge.NewMockDriver(readerName), nil
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}