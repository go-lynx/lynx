package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
)

// GetConfig method is used to obtain configuration from the Polaris configuration center
func (p *PlugPolaris) GetConfig(fileName string, group string) (config.Source, error) {
	// Call the GetPolaris() function to obtain a Polaris instance and use the WithConfigFile method to set the configuration file information
	return GetPolaris().Config(
		polaris.WithConfigFile(
			polaris.File{
				// Set the name of the configuration file
				Name: fileName,
				// Set the group name of the configuration file
				Group: group,
			}))
}
