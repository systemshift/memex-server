package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kbinani/screenshot"
)

// Config holds capture configuration
type Config struct {
	MemexURL string
	Interval time.Duration
	Display  int
	UserID   string
}

func main() {
	// Parse flags
	memexURL := flag.String("memex", getEnv("MEMEX_URL", "http://localhost:8080"), "Memex API URL")
	interval := flag.Duration("interval", 30*time.Second, "Capture interval")
	display := flag.Int("display", 0, "Display number to capture")
	userID := flag.String("user", getEnv("USER", "unknown"), "User identifier")
	flag.Parse()

	config := Config{
		MemexURL: *memexURL,
		Interval: *interval,
		Display:  *display,
		UserID:   *userID,
	}

	// Check display count
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		log.Fatal("No active displays found")
	}
	if config.Display >= n {
		log.Fatalf("Display %d not available (only %d displays)", config.Display, n)
	}

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	log.Printf("memex-capture started (interval=%v, user=%s, display=%d)", config.Interval, config.UserID, config.Display)
	log.Printf("Posting to: %s", config.MemexURL)

	// Capture immediately on start
	if err := captureAndPost(config); err != nil {
		log.Printf("Initial capture error: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := captureAndPost(config); err != nil {
				log.Printf("Capture error: %v", err)
			}
		case <-stop:
			log.Println("Shutting down...")
			return
		}
	}
}

func captureAndPost(config Config) error {
	// Capture screenshot
	bounds := screenshot.GetDisplayBounds(config.Display)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return fmt.Errorf("capture failed: %w", err)
	}

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return fmt.Errorf("encode failed: %w", err)
	}

	// Create node request
	now := time.Now()
	nodeID := fmt.Sprintf("screenshot:%s:%d", config.UserID, now.UnixNano())

	node := map[string]interface{}{
		"id":   nodeID,
		"type": "Screenshot",
		"content": base64.StdEncoding.EncodeToString(buf.Bytes()),
		"meta": map[string]interface{}{
			"status":    "pending",
			"user":      config.UserID,
			"timestamp": now.Format(time.RFC3339),
			"display":   config.Display,
			"width":     bounds.Dx(),
			"height":    bounds.Dy(),
		},
	}

	// POST to memex
	body, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	resp, err := http.Post(
		config.MemexURL+"/api/nodes",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("post failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("memex returned %d", resp.StatusCode)
	}

	log.Printf("Captured: %s (%dx%d, %d bytes)", nodeID, bounds.Dx(), bounds.Dy(), buf.Len())
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
