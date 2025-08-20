package polaris

// GetNamespace method is used to get the namespace corresponding to the PlugPolaris instance.
// Namespaces are typically used in Polaris to isolate configurations and services for different environments or businesses.
// This method obtains the PlugPolaris plugin instance by calling the GetPlugin function,
// then extracts the namespace information from the configuration of that instance.
// The return value is a string type, representing the obtained namespace.
func (p *PlugPolaris) GetNamespace() string {
	if p.conf != nil {
		return p.conf.Namespace
	}
	return "default"
}
