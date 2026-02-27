package ca

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-lynx/lynx/tls"
	"github.com/spf13/cobra"
)

// CmdGenCA generates a shared root CA for the mesh (cert + key PEM files or config snippet).
// Run once; then either mount files or paste --out-config content into your config center.
var CmdGenCA = &cobra.Command{
	Use:   "gen-ca",
	Short: "Generate shared root CA for mesh TLS",
	Long: `Generate a shared root CA (certificate + private key) for the service mesh.

Output options:
  1) --out-dir or --cert/--key: write PEM files, then use shared_ca.from=file.
  2) --out-config yaml: print YAML snippet to paste into config center, then use shared_ca.from=control_plane with config_name=<name>.

Example (file):
  lynx gen-ca --out-dir /etc/lynx-mesh-ca
  lynx gen-ca --cert ./ca.pem --key ./ca-key.pem

Example (config center):
  lynx gen-ca --out-config yaml
  # Copy output into config center under key e.g. lynx-mesh-ca (content: crt + key as below).
  # Service config: shared_ca.from=control_plane, config_name=lynx-mesh-ca`,
	RunE: runGenCA,
}

var (
	genCAOutDir    string
	genCACert      string
	genCAKey       string
	genCAOutConfig string
)

func init() {
	CmdGenCA.Flags().StringVar(&genCAOutDir, "out-dir", "", "Output directory: write ca.pem and ca-key.pem here")
	CmdGenCA.Flags().StringVar(&genCACert, "cert", "", "Output path for CA certificate (PEM)")
	CmdGenCA.Flags().StringVar(&genCAKey, "key", "", "Output path for CA private key (PEM)")
	CmdGenCA.Flags().StringVar(&genCAOutConfig, "out-config", "", "Output for config center: 'yaml' prints crt/key YAML to paste into config center (config_name e.g. lynx-mesh-ca)")
}

func runGenCA(cmd *cobra.Command, args []string) error {
	caCertPEM, caKeyPEM, err := tls.GenerateCAOnly()
	if err != nil {
		return fmt.Errorf("generate CA: %w", err)
	}

	// Output for config center: print YAML snippet only, no files
	if genCAOutConfig != "" {
		if genCAOutDir != "" || genCACert != "" || genCAKey != "" {
			return fmt.Errorf("do not use --out-dir/--cert/--key together with --out-config")
		}
		switch strings.ToLower(genCAOutConfig) {
		case "yaml":
			fmt.Fprintln(cmd.OutOrStdout(), "# Paste this content into your config center as the value for config_name (e.g. lynx-mesh-ca).")
			fmt.Fprintln(cmd.OutOrStdout(), "# Then set in each service: lynx.tls.auto.shared_ca.from=control_plane, config_name=lynx-mesh-ca")
			fmt.Fprintln(cmd.OutOrStdout(), "---")
			fmt.Fprintf(cmd.OutOrStdout(), "crt: |\n%s", indentPEM(caCertPEM))
			fmt.Fprintf(cmd.OutOrStdout(), "key: |\n%s", indentPEM(caKeyPEM))
			return nil
		default:
			return fmt.Errorf("--out-config must be 'yaml'")
		}
	}

	// File output
	if genCAOutDir != "" {
		if genCACert != "" || genCAKey != "" {
			return fmt.Errorf("use either --out-dir or --cert/--key, not both")
		}
		genCACert = filepath.Join(genCAOutDir, "ca.pem")
		genCAKey = filepath.Join(genCAOutDir, "ca-key.pem")
	}
	if genCACert == "" || genCAKey == "" {
		return fmt.Errorf("specify --out-dir, both --cert and --key, or --out-config yaml")
	}

	dir := filepath.Dir(genCACert)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create output dir %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(genCACert, caCertPEM, 0644); err != nil {
		return fmt.Errorf("write CA cert to %s: %w", genCACert, err)
	}
	dirKey := filepath.Dir(genCAKey)
	if dirKey != "." {
		if err := os.MkdirAll(dirKey, 0755); err != nil {
			return fmt.Errorf("create output dir %s: %w", dirKey, err)
		}
	}
	if err := os.WriteFile(genCAKey, caKeyPEM, 0600); err != nil {
		return fmt.Errorf("write CA key to %s: %w", genCAKey, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Shared root CA generated:\n  cert: %s\n  key:  %s\n", genCACert, genCAKey)
	fmt.Fprintln(cmd.OutOrStdout(), "Use in config: lynx.tls.auto.shared_ca.from=file, cert_file=<cert>, key_file=<key>")
	return nil
}

// indentPEM prefixes each line with two spaces for YAML literal block.
func indentPEM(pem []byte) string {
	var b bytes.Buffer
	for _, line := range bytes.Split(bytes.TrimSpace(pem), []byte("\n")) {
		b.WriteString("  ")
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.String()
}
