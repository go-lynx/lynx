package boot

import (
	"github.com/go-lynx/lynx/plugin"
	"sync"
)

type ByWeight []plugin.Plugin

func (a ByWeight) Len() int           { return len(a) }
func (a ByWeight) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByWeight) Less(i, j int) bool { return a[i].Weight() > a[j].Weight() }

var (
	pluginMap     = make(map[string]plugin.Plugin)
	pluginMapLock = sync.RWMutex{}
)

func (a *App) loadingPlugin(b *Lynx) {
	if a.p == nil || len(a.p) == 0 {
		return
	}

	// Load plugins based on weight
	for i := 0; i < len(a.p); i++ {
		a.pluginCheck(i)
		p, err := a.p[i].Load(b)
		if err != nil {
			dfLog.Errorf("Exception in initializing %v plugin :", a.p[i].Name(), err)
			panic(err)
		}

		pluginMapLock.Lock()
		a.pluginCheck(i)
		pluginMap[p.Name()] = p
		pluginMapLock.Unlock()
	}
}

func (a *App) pluginCheck(i int) {
	// Check for duplicate plugin names
	if existingPlugin, exists := pluginMap[a.p[i].Name()]; exists {
		dfLog.Errorf("Duplicate plugin name: %v . Existing Plugin: %v", a.p[i].Name(), existingPlugin)
		panic("Duplicate plugin name: " + a.p[i].Name())
	}
}

func (a *App) cleanPlugin() {
	for i := 0; i < len(a.p); i++ {
		err := a.p[i].Unload()
		if err != nil {
			dfLog.Errorf("Exception in uninstalling %v plugin", a.p[i].Name(), err)
		}
	}
}

func GetPlugin(name string) plugin.Plugin {
	return pluginMap[name]
}
