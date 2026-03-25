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

func TestNewDefaultRuntime(t *testing.T) {
	rt := NewDefaultRuntime()
	if rt == nil {
		t.Fatal("expected explicit runtime to be created")
	}
}

func TestTypedResourceHelpersWithExplicitRuntime(t *testing.T) {
	rt := NewDefaultRuntime()
	if err := RegisterTypedResourceOnRuntime[string](rt, "greeting", "hello"); err != nil {
		t.Fatalf("failed to register typed resource on explicit runtime: %v", err)
	}

	got, err := GetTypedResourceFromRuntime[string](rt, "greeting")
	if err != nil {
		t.Fatalf("failed to get typed resource from explicit runtime: %v", err)
	}
	if got != "hello" {
		t.Fatalf("unexpected resource value: got %q want %q", got, "hello")
	}
}

func TestTypedResourceHelpersRejectNilRuntime(t *testing.T) {
	if err := RegisterTypedResourceOnRuntime[string](nil, "greeting", "hello"); err == nil {
		t.Fatal("expected explicit runtime registration to reject nil runtime")
	}
	if _, err := GetTypedResourceFromRuntime[string](nil, "greeting"); err == nil {
		t.Fatal("expected explicit runtime lookup to reject nil runtime")
	}
}
