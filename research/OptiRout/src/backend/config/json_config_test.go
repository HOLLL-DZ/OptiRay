package config

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestJSONConfigurationRoundTrip(t *testing.T) {
	input := `{"configVersion":"0.8.2","app":{"dnsProxy":{"timeout":"5s","disableDropAAAA":true},"netfilter":{"ipset":{"additionalTTL":"1h0m0s"}}},"groups":[{"id":"1234abcd","name":"Тест","enable":true,"rules":[{"id":"fedc4321","type":"domain","rule":"example.com","enable":true}]}],"subscriptions":[]}`
	var config Config
	if err := json.Unmarshal([]byte(input), &config); err != nil {
		t.Fatal(err)
	}
	if *config.App.DNSProxy.Timeout != 5*time.Second || *config.App.Netfilter.IPSet.AdditionalTTL != time.Hour {
		t.Fatal("duration changed")
	}
	output, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), `"id":"1234abcd"`) || !strings.Contains(string(output), `"timeout":"5s"`) {
		t.Fatal("ID or duration representation changed")
	}
	var restored Config
	if err := json.Unmarshal(output, &restored); err != nil || !reflect.DeepEqual(config, restored) {
		t.Fatalf("round trip changed configuration: %v", err)
	}
}

func TestJSONDurationsCompatibility(t *testing.T) {
	for _, input := range []string{`{"timeout":5000000000}`, `{"timeout":"5s"}`} {
		var settings DNSProxy
		if err := json.Unmarshal([]byte(input), &settings); err != nil || *settings.Timeout != 5*time.Second {
			t.Fatalf("incompatible duration: %s (%v)", input, err)
		}
	}
	var settings DNSProxy
	if err := json.Unmarshal([]byte(`{"timeout":"invalid"}`), &settings); err == nil {
		t.Fatal("invalid duration accepted")
	}
}

func TestInstallationConfiguration(t *testing.T) {
	path := os.Getenv("OPTIRAY_TEST_CONFIG")
	if path == "" {
		t.Skip("no installation fixture")
	}
	input, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config Config
	if err := json.Unmarshal(input, &config); err != nil {
		t.Fatal(err)
	}
	output, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var restored Config
	if err := json.Unmarshal(output, &restored); err != nil || !reflect.DeepEqual(config, restored) {
		t.Fatalf("installation configuration changed: %v", err)
	}
}
