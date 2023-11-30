package app

import "github.com/go-lynx/lynx/conf"

func (a *LynxApp) Tls() *conf.Tls {
	return Lynx().tls
}
