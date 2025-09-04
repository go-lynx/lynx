package envx

import (
	"testing"
	"time"
)

func TestEnvX(t *testing.T) {
	t.Setenv("APP_NAME", "lynx")
	if Get("APP_NAME", "def") != "lynx" {
		t.Fatalf("Get string")
	}
	if Get("APP_MISS", "def") != "def" {
		t.Fatalf("Get default")
	}

	t.Setenv("PORT", "8081")
	if GetInt("PORT", 80) != 8081 {
		t.Fatalf("GetInt")
	}
	if GetInt("PORT_BAD", 80) != 80 {
		t.Fatalf("GetInt default")
	}

	t.Setenv("DEBUG", "true")
	if !GetBool("DEBUG", false) {
		t.Fatalf("GetBool")
	}
	if GetBool("DEBUG_BAD", true) != true {
		t.Fatalf("GetBool default")
	}

	t.Setenv("TIMEOUT", "150ms")
	if GetDuration("TIMEOUT", time.Second) != 150*time.Millisecond {
		t.Fatalf("GetDuration parse")
	}
	t.Setenv("TIMEOUT", "2")
	if GetDuration("TIMEOUT", time.Second) != 2*time.Second {
		t.Fatalf("GetDuration seconds")
	}
}
