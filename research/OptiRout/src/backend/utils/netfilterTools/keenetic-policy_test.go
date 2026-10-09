package netfilterTools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestPolicyScope(t *testing.T) {
	s := policyScope{0xffffaab: {"nwg0": true}, 0xffffaaa: {"eth3": true}, 0xffffaac: {"t2s0": true}}
	owned := map[uint32]string{0x4d616769: "t2s0", 0x4d61676a: "nwg0"}
	if got := deniedPolicyMarks(s, "t2s0", owned); !reflect.DeepEqual(got, []uint32{0xffffaaa, 0xffffaab, 0x4d61676a}) {
		t.Fatalf("wrong exclusions: %x", got)
	}
	delete(s[0xffffaac], "t2s0")
	if len(deniedPolicyMarks(s, "t2s0", owned)) != 4 {
		t.Fatal("disabled proxy still permitted")
	}
}
func TestPolicyRCI(t *testing.T) {
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(503)
			return
		}
		switch r.URL.Path {
		case "/rci/ip/policy":
			w.Write([]byte(`{"Policy0":{"permit":[{"interface":"Wireguard0","enabled":true},{"interface":"Proxy0","enabled":false,"no":true}]},"Policy1":{"permit":[{"interface":"Proxy0","enabled":true}]}}`))
		case "/rci/show/ip/policy":
			w.Write([]byte(`{"Policy0":{"mark":"ffffaab"},"Policy1":{"mark":"ffffaac"}}`))
		case "/rci/":
			var reqs []struct {
				Show struct {
					Interface struct {
						Name string `json:"name"`
					} `json:"interface"`
				} `json:"show"`
			}
			json.NewDecoder(r.Body).Decode(&reqs)
			result := []any{}
			for _, v := range reqs {
				name := "t2s0"
				if v.Show.Interface.Name == "Wireguard0" {
					name = "nwg0"
				}
				result = append(result, map[string]any{"show": map[string]any{"interface": map[string]string{"system-name": name}}})
			}
			json.NewEncoder(w).Encode(result)
		default:
			t.Errorf("unknown endpoint %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	g := newPolicyGuard()
	g.baseURL = srv.URL
	g.client = srv.Client()
	s, err := g.load()
	if err != nil {
		t.Fatal(err)
	}
	if s[0xffffaab]["t2s0"] || !s[0xffffaab]["nwg0"] || !s[0xffffaac]["t2s0"] {
		t.Fatalf("bad mapping: %#v", s)
	}
	fail = true
	if _, err := g.load(); err == nil {
		t.Fatal("RCI failure ignored")
	}
}
