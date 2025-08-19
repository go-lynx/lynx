package log

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	kconf "github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app/conf"
)

//go:generate echo "banner.go"

// initBanner initializes and displays the application banner.
// It first attempts to read from a local banner file, then falls back to an embedded banner.
// The banner display can be disabled through application configuration.
func initBanner(cfg kconf.Config) error {
	const (
		localBannerPath    = "configs/banner.txt"
		embeddedBannerPath = "banner.txt"
	)

	// Try to read banner data, with fallback options
	bannerData, err := loadBannerData(localBannerPath)
	if err != nil {
		// Log the local file read failure and try embedded banner
		Debugf("could not read local banner: %v, falling back to embedded banner", err)
		bannerData, err = fs.ReadFile(bannerFS, embeddedBannerPath)
		if err != nil {
			return fmt.Errorf("failed to read embedded banner: %v", err)
		}
	}

	// Parse application configuration
	var bootConfig conf.Bootstrap
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

// Embedded banner file for application startup
// 使用 //go:embed 指令将 banner.txt 文件嵌入到程序中
//
//go:embed banner.txt
var bannerFS embed.FS

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
