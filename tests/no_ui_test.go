package tests

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCrudImportsNoUI(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "webtyp.com/crud")
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, f := range []string{"webtyp.com/dom", "webtyp.com/html", "webtyp.com/form", "webtyp.com/layout", "webtyp.com/svg", "webtyp.com/view"} {
		for _, line := range strings.Split(string(out), "\n") {
			if line == f {
				t.Fatalf("crud must not import UI package %s: servers and edge Workers import crud", f)
			}
		}
	}
}
