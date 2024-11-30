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
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	// Get Bluesky credentials from environment
	identifier := os.Getenv("BLUESKY_IDENTIFIER")
	password := os.Getenv("BLUESKY_PASSWORD")
	if identifier == "" || password == "" {
		log.Fatal("BLUESKY_IDENTIFIER and BLUESKY_PASSWORD must be set")
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

	// Create a new cron scheduler
	c := cron.New()

	// Schedule random palette posts every 6 hours
	_, err = c.AddFunc("0 */6 * * *", func() {
		log.Println("Generating and posting new color palette...")
		if err := b.GenerateAndPost(ctx); err != nil {
			log.Printf("Error posting palette: %v", err)
		}
	})
	if err != nil {
		log.Fatal("Failed to schedule random palette cron job:", err)
	}

	// Schedule Bing image palette post once per day at 12:00 PM
	_, err = c.AddFunc("0 12 * * *", func() {
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

	// Generate and post initial random palette
	log.Println("Generating and posting initial random color palette...")
	if err := b.GenerateAndPost(ctx); err != nil {
		log.Printf("Error posting initial palette: %v", err)
	}

	// Keep the program running
	for {
		time.Sleep(time.Hour)
	}
}
