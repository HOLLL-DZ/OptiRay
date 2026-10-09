package config

import (
	"encoding/json"
	"strconv"
	"time"
)

// Keep human-readable durations and accept the older numeric representation.
type jsonDuration time.Duration

func (d jsonDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *jsonDuration) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		value, err := time.ParseDuration(text)
		if err != nil {
			return err
		}
		*d = jsonDuration(value)
		return nil
	}
	value, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*d = jsonDuration(value)
	return nil
}

type dnsProxyJSON DNSProxy

func (c DNSProxy) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		*dnsProxyJSON
		Timeout *jsonDuration `json:"timeout"`
	}{(*dnsProxyJSON)(&c), (*jsonDuration)(c.Timeout)})
}

func (c *DNSProxy) UnmarshalJSON(data []byte) error {
	value := struct {
		*dnsProxyJSON
		Timeout *jsonDuration `json:"timeout"`
	}{dnsProxyJSON: (*dnsProxyJSON)(c)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	c.Timeout = (*time.Duration)(value.Timeout)
	return nil
}

type ipSetJSON IPSet

func (c IPSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		*ipSetJSON
		AdditionalTTL *jsonDuration `json:"additionalTTL"`
	}{(*ipSetJSON)(&c), (*jsonDuration)(c.AdditionalTTL)})
}

func (c *IPSet) UnmarshalJSON(data []byte) error {
	value := struct {
		*ipSetJSON
		AdditionalTTL *jsonDuration `json:"additionalTTL"`
	}{ipSetJSON: (*ipSetJSON)(c)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	c.AdditionalTTL = (*time.Duration)(value.AdditionalTTL)
	return nil
}
