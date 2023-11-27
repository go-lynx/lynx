package plugin

type Plugin interface {
	Weight() int
	Name() string
	Load(config interface{}) (Plugin, error)
	Unload() error
}
