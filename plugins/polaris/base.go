package polaris

func (p *PlugPolaris) GetNamespace() string {
	return GetPlugin().conf.GetNamespace()
}
