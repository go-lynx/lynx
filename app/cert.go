package app

type Cert interface {
	Crt() []byte
	Key() []byte
	RootCA() []byte
}

func (a *LynxApp) Cert() Cert {
	return a.cert
}

func (a *LynxApp) SetCert(cert Cert) {
	a.cert = cert
}
