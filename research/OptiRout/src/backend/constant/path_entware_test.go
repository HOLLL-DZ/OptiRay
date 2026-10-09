//go:build entware

package constant

import "testing"

func TestEntwarePathOverride(t *testing.T) {
	t.Setenv("OPTIRAY_ROUTING_TEST_PATH", "/isolated/path")
	if got := entwarePath("OPTIRAY_ROUTING_TEST_PATH", "/default"); got != "/isolated/path" {
		t.Fatal(got)
	}
	if got := entwarePath("OPTIRAY_ROUTING_TEST_ABSENT", "/default"); got != "/default" {
		t.Fatal(got)
	}
}
