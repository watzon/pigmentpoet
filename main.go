package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"github.com/watzon/pigmentpoet/bot"
)

func main() {
	// Load environment variables
	_ = godotenv.Load()

	// Get Bluesky credentials from environment
	identifier := os.Getenv("BLUESKY_IDENTIFIER")
	password := os.Getenv("BLUESKY_PASSWORD")
	if identifier == "" || password == "" {
		log.Fatal("BLUESKY_IDENTIFIER and BLUESKY_PASSWORD must be set")
	}

	// Get timezone from environment or default to UTC
	timezone := os.Getenv("TZ")
	location, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("Warning: Invalid timezone %q, defaulting to UTC", timezone)
		location = time.UTC
	}

	// Create output directory for palette images
	outputDir := filepath.Join(os.TempDir(), "pigmentpoet")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal("Failed to create output directory:", err)
	}

	// Create bot instance
	ctx := context.Background()
	b, err := bot.NewBot(ctx, identifier, password, outputDir)
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}

	// Create a new cron scheduler with configured timezone
	c := cron.New(cron.WithLocation(location))

	// Schedule token refresh every hour
	_, err = c.AddFunc("0 * * * *", func() {
		log.Println("Refreshing authentication token...")
		if err := b.RefreshSession(ctx); err != nil {
			log.Printf("Error refreshing token: %v", err)
		}
	})
	if err != nil {
		log.Fatal("Failed to schedule token refresh cron job:", err)
	}

	// Schedule random palette posts every 6 hours
	_, err = c.AddFunc("0 */6 * * *", func() {
		log.Println("Generating and posting new color palette...")
		if err := b.GenerateAndPost(ctx, nil); err != nil {
			log.Printf("Error posting palette: %v", err)
		}
	})
	if err != nil {
		log.Fatal("Failed to schedule random palette cron job:", err)
	}

	// Schedule Bing image palette post once per day at 11:00 AM
	_, err = c.AddFunc("0 11 * * *", func() {
		log.Println("Generating and posting palette from Bing image of the day...")
		if err := b.GenerateAndPostFromBing(ctx); err != nil {
			log.Printf("Error posting Bing palette: %v", err)
		}
	})
	if err != nil {
		log.Fatal("Failed to schedule Bing palette cron job:", err)
	}

	// Start the scheduler
	c.Start()

	// Start firehose listener
	log.Println("Starting firehose listener...")
	if err := b.StartFirehoseListener(ctx); err != nil {
		log.Printf("Error starting firehose listener: %v", err)
	}

	// // Generate and post initial random palette
	// log.Println("Generating and posting initial random color palette...")
	// if err := b.GenerateAndPost(ctx); err != nil {
	// 	log.Printf("Error posting initial palette: %v", err)
	// }

	// Keep the program running
	for {
		time.Sleep(time.Hour)
	}
}
