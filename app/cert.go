package app

import (
	"github.com/go-lynx/lynx/plugin/cert/conf"
)

func (a *LynxApp) Cert() *conf.Cert {
	return a.cert
}

func (a *LynxApp) SetCert(cert *conf.Cert) {
	a.cert = cert
}
