package polaris

// GetNamespace 方法用于获取 PlugPolaris 实例对应的命名空间。
// 命名空间通常用于在 Polaris 中隔离不同环境或业务的配置和服务。
// 该方法通过调用 GetPlugin 函数获取 PlugPolaris 插件实例，
// 然后从该实例的配置中提取命名空间信息。
// 返回值为字符串类型，表示获取到的命名空间。
func (p *PlugPolaris) GetNamespace() string {
	if p.conf != nil {
		return p.conf.Namespace
	}
	return "default"
}
