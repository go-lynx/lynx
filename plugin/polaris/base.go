package polaris

func (p *PlugPolaris) Namespace() string {
	return GetPlugPolaris().conf.GetNamespace()
}
