package lynx

import "testing"

func TestTypedRuntimePluginUnderlyingRuntime(t *testing.T) {
	rt := NewTypedRuntimePlugin()
	if rt == nil {
		t.Fatal("expected runtime wrapper to be created")
	}
	if rt.UnderlyingRuntime() == nil {
		t.Fatal("expected underlying runtime to be exposed")
	}
}

func TestTypedRuntimePluginUnderlyingRuntimeNilReceiver(t *testing.T) {
	var rt *TypedRuntimePlugin
	if got := rt.UnderlyingRuntime(); got != nil {
		t.Fatalf("expected nil underlying runtime for nil receiver, got %#v", got)
	}
}
