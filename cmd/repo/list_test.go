package repo

import (
	"reflect"
	"testing"

	"github.com/kohbis/xr/internal/config"
)

func TestFilterRepos(t *testing.T) {
	repos := []config.Repository{
		{Name: "api"},
		{Name: "web"},
		{Name: "worker"},
	}

	tests := []struct {
		name  string
		names []string
		want  []string
	}{
		{name: "single match", names: []string{"web"}, want: []string{"web"}},
		{name: "repos.yaml order wins over flag order", names: []string{"worker", "api"}, want: []string{"api", "worker"}},
		{name: "unknown name matches nothing", names: []string{"missing"}, want: []string{}},
		{name: "duplicate name is not repeated", names: []string{"api", "api"}, want: []string{"api"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterRepos(repos, tt.names)
			gotNames := make([]string, 0, len(got))
			for _, r := range got {
				gotNames = append(gotNames, r.Name)
			}
			if !reflect.DeepEqual(gotNames, tt.want) {
				t.Fatalf("filterRepos() = %v, want %v", gotNames, tt.want)
			}
		})
	}
}
