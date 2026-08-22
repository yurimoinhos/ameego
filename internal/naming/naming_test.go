package naming_test

import (
	"testing"

	"github.com/yurimoinhos/ameego/internal/naming"
)

func TestPluralStructName(t *testing.T) {
	cases := map[string]string{
		"users":  "Users",
		"roles":  "Roles",
		"orders": "Orders",
	}
	for in, want := range cases {
		if got := naming.PluralStructName(in); got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
}
