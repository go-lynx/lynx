package subscribe

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins/tls/conf"
)

// tlsLoad 方法用于加载 TLS 配置。如果未启用 TLS，则返回 nil。
// 该方法会尝试获取根证书并将其添加到证书池中，最终返回一个配置好的 tls.Config 实例。
func (g *GrpcSubscribe) tlsLoad() *tls.Config {
	// 检查是否启用 TLS，如果未启用则直接返回 nil
	if !g.tls {
		return nil
	}

	// 创建一个新的证书池，用于存储根证书
	certPool := x509.NewCertPool()
	var rootCA []byte

	// 检查是否指定了根 CA 证书的名称
	if g.caName != "" {
		// Obtain the root certificate of the remote file
		// 检查应用的控制平面是否可用，如果不可用则返回 nil
		if app.Lynx().GetControlPlane() == nil {
			return nil
		}
		// if group is empty, use the name as the group name.
		// 如果未指定根 CA 证书文件所属的组，则使用根 CA 证书的名称作为组名
		if g.caGroup == "" {
			g.caGroup = g.caName
		}
		// 从控制平面获取配置信息
		s, err := app.Lynx().GetControlPlane().GetConfig(g.caName, g.caGroup)
		if err != nil {
			// 若获取配置信息失败，则触发 panic
			panic(err)
		}
		// 创建一个新的配置实例，并将从控制平面获取的配置源设置进去
		c := config.New(
			config.WithSource(s),
		)
		// 加载配置信息
		if err := c.Load(); err != nil {
			// 若加载配置信息失败，则触发 panic
			panic(err)
		}
		// 定义一个 Cert 结构体变量，用于存储从配置中扫描出的证书信息
		var t conf.Cert
		// 将配置信息扫描到 Cert 结构体变量中
		if err := c.Scan(&t); err != nil {
			// 若扫描配置信息失败，则触发 panic
			panic(err)
		}
		// 将从配置中获取的根 CA 证书信息转换为字节切片
		rootCA = []byte(t.GetRootCA())
	} else {
		// Use the root certificate of the current application directly
		// 若未指定根 CA 证书的名称，则直接使用当前应用的根证书
		rootCA = app.Lynx().Cert().GetRootCA()
	}
	// 将根证书添加到证书池中，如果添加失败则触发 panic
	if !certPool.AppendCertsFromPEM(rootCA) {
		panic("Failed to load root certificate")
	}
	// 返回配置好的 TLS 配置实例，设置服务器名称和根证书池
	return &tls.Config{ServerName: g.caName, RootCAs: certPool}
}
