package base

import (
	"strings"
	"testing"
)

func TestApplyReplacesToGeneratedProto_KeepsRawDescIntact(t *testing.T) {
	src := strings.Join([]string{
		`package v1`,
		`import (`,
		`	common "github.com/go-lynx/lynx-layout/api/common"`,
		`)`,
		`const file_login_proto_rawDesc = "" +`,
		`	"\n" +`,
		`	"Z.github.com/go-lynx/lynx-layout/api/login/v1;v1b\x06proto3"`,
		`var _ = common.X`,
		`// see github.com/go-lynx/lynx-layout`,
	}, "\n")
	got := string(applyReplacesToGeneratedProto([]byte(src), []string{"github.com/go-lynx/lynx-layout", "example.com/demo"}))
	if !strings.Contains(got, `common "example.com/demo/api/common"`) {
		t.Fatalf("import not rewritten:\n%s", got)
	}
	if !strings.Contains(got, `"Z.github.com/go-lynx/lynx-layout/api/login/v1;v1b\x06proto3"`) {
		t.Fatalf("rawDesc was modified:\n%s", got)
	}
	if !strings.Contains(got, `// see example.com/demo`) {
		t.Fatalf("trailing comment not rewritten:\n%s", got)
	}
}
