package app

type Cert interface {
	GetCrt() []byte
	GetKey() []byte
	GetRootCA() []byte
}

func (a *LynxApp) Cert() Cert {
	return a.cert
}

func (a *LynxApp) SetCert(cert Cert) {
	a.cert = cert
}
