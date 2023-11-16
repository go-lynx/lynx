package boot

import (
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plug"
	"sort"
)

type ByWeight []plug.Plug

func (a ByWeight) Len() int           { return len(a) }
func (a ByWeight) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByWeight) Less(i, j int) bool { return a[i].Weight() > a[j].Weight() }

var plugMap = make(map[string]plug.Plug)

func (a *App) loadingPlug(b *conf.Bootstrap) {
	if a.p == nil {
		return
	}
	if len(a.p) == 0 {
		return
	}
	// plug weight sort
	sort.Sort(ByWeight(a.p))
	for i := 0; i < len(a.p); i++ {
		p, err := a.p[i].Load(b)
		if err != nil {
			dfLog.Errorf("Exception in initializing %v plugin :", a.p[i].Name(), err)
			panic(err)
		}
		plugMap[p.Name()] = p
	}
}

func (a *App) cleanPlug() {
	for i := 0; i < len(a.p); i++ {
		err := a.p[i].Unload()
		if err != nil {
			dfLog.Errorf("Exception in uninstalling %v plugin", a.p[i].Name(), err)
			panic(err)
		}
	}
}

func GetPlug(name string) plug.Plug {
	return plugMap[name]
}
