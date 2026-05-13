package main

import (
	"encoding/json"
	"testing"
)

func TestCloudflareRecordName(t *testing.T) {
	tests := []struct {
		name       string
		domainName string
		recordName string
		want       string
	}{
		{
			name:       "subdomain",
			domainName: "example.com",
			recordName: "home",
			want:       "home.example.com",
		},
		{
			name:       "apex",
			domainName: "example.com",
			recordName: "@",
			want:       "example.com",
		},
		{
			name:       "fully qualified",
			domainName: "example.com",
			recordName: "home.example.com",
			want:       "home.example.com",
		},
		{
			name:       "trailing dots",
			domainName: "example.com.",
			recordName: "home.example.com.",
			want:       "home.example.com",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := cloudflareRecordName(test.domainName, test.recordName)
			if got != test.want {
				t.Fatalf("cloudflareRecordName(%q, %q) = %q, want %q", test.domainName, test.recordName, got, test.want)
			}
		})
	}
}

func TestCloudflareRecordPayloadOmitsUnsetProxied(t *testing.T) {
	payload := cloudflareRecordPayload{
		Type:    "A",
		Name:    "home.example.com",
		Content: "192.0.2.1",
		TTL:     300,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"type":"A","name":"home.example.com","content":"192.0.2.1","ttl":300}` {
		t.Fatalf("payload JSON = %s", string(body))
	}
}

func TestOptionalBoolFlag(t *testing.T) {
	var value optionalBoolFlag
	if value.valuePtr() != nil {
		t.Fatal("unset optionalBoolFlag returned a value pointer")
	}
	if err := value.Set("false"); err != nil {
		t.Fatal(err)
	}
	if value.valuePtr() == nil || *value.valuePtr() {
		t.Fatalf("optionalBoolFlag = %#v, want explicit false", value)
	}
}
