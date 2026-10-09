package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInterfaceDescriptionsDecodeNDMCEscapedUTF8(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"br0", "nwg0", "t2s0"} {
		if e := os.Mkdir(filepath.Join(dir, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	text := "interface Bridge0\n description \"Wi-Fi " + `\xd0\x94\xd0\xbe\xd0\xbc` + "\"\n!\ninterface Wireguard0\n description Admin [" + `\xd0\x9c\xd0\xb0\xd1\x8f\xd0\xba` + "]\n!\ninterface Proxy0\n description OptiRay_Admin\n!\n"
	items, e := collectInterfaces(dir, text)
	if e != nil {
		t.Fatal(e)
	}
	names := map[string]string{}
	for _, item := range items {
		m := obj(item)
		names[str(m["name"])] = str(m["desc"])
	}
	if names["br0"] != "Wi-Fi Дом" || names["nwg0"] != "Admin [Маяк]" || names["t2s0"] != "OptiRay_Admin" {
		t.Fatal(names)
	}
	for _, value := range []string{"Дом", `bad \xd0`, `keep \xZZ`, "ASCII"} {
		if got := decodeInterfaceDescription(value); got != value {
			t.Fatalf("altered intact/invalid label: %q", got)
		}
	}
}
