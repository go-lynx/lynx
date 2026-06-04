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

	// Prefer a project-local banner; fall back to the embedded default.
	bannerData, err := loadBannerData(localBannerPath)
	if err != nil {
		if data, e := fs.ReadFile(bannerFS, embeddedBannerPath); e == nil {
			bannerData = data
		} else {
			return fmt.Errorf("failed to read banner: local=%v, embedded=%v", err, e)
		}
	}

	var bootConfig appconf.Bootstrap
	if err := cfg.Scan(&bootConfig); err != nil {
		return fmt.Errorf("failed to parse configuration: %v", err)
	}

	app := bootConfig.GetLynx().GetApplication()
	if app == nil {
		return fmt.Errorf("invalid configuration: application settings not found")
	}

	if !app.GetCloseBanner() {
		if err := displayBanner(bannerData); err != nil {
			return fmt.Errorf("failed to display banner: %v", err)
		}
	}

	return nil
}

// loadBannerData reads banner content from path, returning an error if it is
// missing or unreadable so the caller can fall back to the embedded banner.
func loadBannerData(path string) ([]byte, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read banner file: %v", err)
	}

	return data, nil
}

// displayBanner writes the banner data to standard output.
// It returns an error if the write operation fails.
// Uses Fprint (not Fprintln) because banner content already includes trailing newline.
func displayBanner(data []byte) error {
	_, err := fmt.Fprint(os.Stdout, string(data))
	return err
}
