package repo

import (
	"strings"
	"testing"
)

func TestResolveSource_Protocol(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		protocol string
		want     string
		wantErr  string
	}{
		{name: "org/repo defaults to ssh", source: "acme/api", want: "git@github.com:acme/api.git"},
		{name: "org/repo over https", source: "acme/api.git", protocol: "https", want: "https://github.com/acme/api.git"},
		{name: "org/repo over ssh", source: "acme/api", protocol: "ssh", want: "git@github.com:acme/api.git"},

		{name: "https URL alone", source: "https://github.com/acme/api.git", want: "https://github.com/acme/api.git"},
		{name: "https URL, matching protocol", source: "https://github.com/acme/api.git", protocol: "https", want: "https://github.com/acme/api.git"},
		{name: "scp URL, matching protocol", source: "git@github.com:acme/api.git", protocol: "ssh", want: "git@github.com:acme/api.git"},
		{name: "ssh:// URL, matching protocol", source: "ssh://git@ghe.example.com/acme/api.git", protocol: "ssh", want: "ssh://git@ghe.example.com/acme/api.git"},

		{name: "https URL, ssh protocol", source: "https://github.com/acme/api.git", protocol: "ssh", wantErr: "contradicts"},
		{name: "scp URL, https protocol", source: "git@github.com:acme/api.git", protocol: "https", wantErr: "contradicts"},
		{name: "file URL with protocol", source: "file:///srv/api.git", protocol: "ssh", wantErr: "does not apply"},
		{name: "local path with protocol", source: "~/src/api", protocol: "https", wantErr: "does not apply"},
		{name: "local path alone", source: "~/src/api", want: "~/src/api"},
		{name: "unknown protocol", source: "acme/api", protocol: "git", wantErr: "must be ssh or https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveSource(tt.source, tt.protocol, false, nil)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveSource() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSource() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveSource() = %q, want %q", got, tt.want)
			}
		})
	}
}
