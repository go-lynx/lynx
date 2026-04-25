package security

import (
	"fmt"
	"os"
	"strings"
)

var productionEnvValues = map[string]struct{}{
	"prod":       {},
	"production": {},
}

// Environment returns the normalized deployment environment.
func Environment() string {
	for _, key := range []string{"LYNX_ENV", "APP_ENV", "GO_ENV", "ENVIRONMENT"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return strings.ToLower(value)
		}
	}
	return ""
}

// IsProduction reports whether the current process is running in a production environment.
func IsProduction() bool {
	_, ok := productionEnvValues[Environment()]
	return ok
}

// ValidateTLSProductionPolicy rejects TLS settings that are unsafe for production.
func ValidateTLSProductionPolicy(component string, enabled bool, insecureSkipVerify bool) error {
	if !enabled || !insecureSkipVerify || !IsProduction() {
		return nil
	}
	if strings.TrimSpace(component) == "" {
		component = "component"
	}
	return fmt.Errorf("%s TLS uses insecure_skip_verify=true in production; set LYNX_ENV to a non-production value for local testing or configure trusted certificates", component)
}
