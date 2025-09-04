package cast

import (
	"errors"
	"testing"
	"time"
)

type s string

func (x s) String() string { return string(x) }

func TestToInt(t *testing.T) {
	v, err := ToInt("12")
	if err != nil || v != 12 {
		t.Fatalf("ToInt string: %v %d", err, v)
	}
	if _, err := ToInt(errors.New("x")); err == nil {
		t.Fatalf("ToInt unsupported should error")
	}
	if ToIntDefault("bad", 9) != 9 {
		t.Fatalf("ToIntDefault fallback")
	}
}

func TestToBool(t *testing.T) {
	b, err := ToBool("true")
	if err != nil || !b {
		t.Fatalf("ToBool true failed")
	}
	if ToBoolDefault("bad", true) != true {
		t.Fatalf("ToBoolDefault fallback")
	}
	b, _ = ToBool(1)
	if !b {
		t.Fatalf("ToBool int -> true")
	}
}

func TestToFloat64(t *testing.T) {
	f, err := ToFloat64("1.5")
	if err != nil || f != 1.5 {
		t.Fatalf("ToFloat64 string failed")
	}
	if ToFloat64Default("bad", 2.3) != 2.3 {
		t.Fatalf("ToFloat64Default fallback")
	}
}

func TestToDuration(t *testing.T) {
	d, err := ToDuration("150ms")
	if err != nil || d != 150*time.Millisecond {
		t.Fatalf("ToDuration parse duration failed")
	}
	d, err = ToDuration("2")
	if err != nil || d != 2*time.Second {
		t.Fatalf("ToDuration numeric seconds failed")
	}
	if ToDurationDefault("bad", time.Minute) != time.Minute {
		t.Fatalf("ToDurationDefault fallback")
	}
}
