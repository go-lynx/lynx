package polaris

func (p *PlugPolaris) Namespace() string {
	return GetPlugin().conf.GetNamespace()
}
