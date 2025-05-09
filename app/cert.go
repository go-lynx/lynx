package app

// Cert 定义了一个证书接口，包含获取证书、私钥和根 CA 证书的方法。
type Cert interface {
	// GetCrt 获取证书的字节切片。
	GetCrt() []byte
	// GetKey 获取私钥的字节切片。
	GetKey() []byte
	// GetRootCA 获取根 CA 证书的字节切片。
	GetRootCA() []byte
}

// Cert 是 LynxApp 结构体的一个方法，用于获取当前应用的证书对象。
// 返回值为实现了 Cert 接口的对象。
func (a *LynxApp) Cert() Cert {
	return a.cert
}

// SetCert 是 LynxApp 结构体的一个方法，用于设置当前应用的证书对象。
// 参数 cert 为实现了 Cert 接口的对象。
func (a *LynxApp) SetCert(cert Cert) {
	a.cert = cert
}
