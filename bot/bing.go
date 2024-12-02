package bot

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"
)

const bingImageURL = "https://www.bing.com/HPImageArchive.aspx?format=js&idx=0&n=1&mkt=en-US"

type bingResponse struct {
	Images []struct {
		URL           string `json:"url"`
		URLBase       string `json:"urlbase"`
		Title         string `json:"title"`
		Copyright     string `json:"copyright"`
		CopyrightLink string `json:"copyrightlink"`
	} `json:"images"`
}

// getBingImageOfDay fetches Bing's image of the day
func getBingImageOfDay() (image.Image, string, string, error) {
	// Fetch image metadata
	resp, err := http.Get(bingImageURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to fetch Bing image metadata: %w", err)
	}
	defer resp.Body.Close()

	var data bingResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, "", "", fmt.Errorf("failed to decode Bing response: %w", err)
	}

	if len(data.Images) == 0 {
		return nil, "", "", fmt.Errorf("no images found in Bing response")
	}

	// Get the base URL and add size parameters for a smaller image
	baseURL := data.Images[0].URLBase
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "https://www.bing.com" + baseURL
	}

	// Request a 1024x768 image size
	imgURL := baseURL + "_1024x768.jpg"

	// Fetch the actual image
	imgResp, err := http.Get(imgURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to fetch image: %w", err)
	}
	defer imgResp.Body.Close()

	// Decode the image
	img, _, err := image.Decode(imgResp.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Resize the image if needed
	img = resizeImage(img)

	return img, data.Images[0].Title, data.Images[0].Copyright, nil
}
