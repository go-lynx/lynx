package banner

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	kconf "github.com/go-kratos/kratos/v2/config"
	appconf "github.com/go-lynx/lynx/conf"
)

// Embedded banner file for application startup
//
//go:embed banner.txt
var bannerFS embed.FS

// Init initializes and displays the application banner.
// It first attempts to read from a local banner file, then falls back to an embedded banner.
// The banner display can be disabled through application configuration.
func Init(cfg kconf.Config) error {
	const (
		localBannerPath    = "configs/banner.txt"
		embeddedBannerPath = "banner.txt"
	)

	// Try to read banner data, with fallback options
	bannerData, err := loadBannerData(localBannerPath)
	if err != nil {
		// Fallback to embedded banner silently
		if data, e := fs.ReadFile(bannerFS, embeddedBannerPath); e == nil {
			bannerData = data
		} else {
			return fmt.Errorf("failed to read banner: local=%v, embedded=%v", err, e)
		}
	}

	// Parse application configuration
	var bootConfig appconf.Bootstrap
	if err := cfg.Scan(&bootConfig); err != nil {
		return fmt.Errorf("failed to parse configuration: %v", err)
	}

	// Validate configuration structure
	app := bootConfig.GetLynx().GetApplication()
	if app == nil {
		return fmt.Errorf("invalid configuration: application settings not found")
	}

	// Display banner unless explicitly disabled
	if !app.GetCloseBanner() {
		if err := displayBanner(bannerData); err != nil {
			return fmt.Errorf("failed to display banner: %v", err)
		}
	}

	return nil
}

// loadBannerData attempts to read banner data from the specified file.
// It returns the banner content as bytes or an error if the read fails.
func loadBannerData(path string) ([]byte, error) {
	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	// Read file contents
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read banner file: %v", err)
	}

	return data, nil
}

// displayBanner writes the banner data to standard output.
// It returns an error if the write operation fails.
func displayBanner(data []byte) error {
	_, err := fmt.Fprintln(os.Stdout, string(data))
	return err
}
