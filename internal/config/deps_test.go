package config

import (
	"strings"
	"testing"
)

func TestSortByDependency(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]Service
		wantErr  string
		validate func([]string) bool
	}{
		{
			name: "no dependencies",
			services: map[string]Service{
				"a": {Command: "x"},
				"b": {Command: "x"},
			},
			validate: func(order []string) bool {
				return len(order) == 2
			},
		},
		{
			name: "linear chain",
			services: map[string]Service{
				"api": {Command: "x", DependsOn: []string{"db"}},
				"db":  {Command: "x"},
			},
			validate: func(order []string) bool {
				return indexOf(order, "db") < indexOf(order, "api")
			},
		},
		{
			name: "multiple dependencies",
			services: map[string]Service{
				"api":   {Command: "x", DependsOn: []string{"db", "redis"}},
				"db":    {Command: "x"},
				"redis": {Command: "x"},
			},
			validate: func(order []string) bool {
				apiIdx := indexOf(order, "api")
				return indexOf(order, "db") < apiIdx && indexOf(order, "redis") < apiIdx
			},
		},
		{
			name: "circular dependency",
			services: map[string]Service{
				"a": {Command: "x", DependsOn: []string{"b"}},
				"b": {Command: "x", DependsOn: []string{"a"}},
			},
			wantErr: "circular dependency",
		},
		{
			name: "unknown dependency",
			services: map[string]Service{
				"a": {Command: "x", DependsOn: []string{"unknown"}},
			},
			wantErr: "unknown service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := SortByDependency(tt.services)
			if tt.wantErr != "" {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.validate(order) {
				t.Errorf("invalid order: %v", order)
			}
		})
	}
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
