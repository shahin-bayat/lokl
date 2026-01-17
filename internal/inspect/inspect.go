// Package inspect provides project detection for lokl init.
package inspect

import "path/filepath"

// Service represents a detected service in the project.
type Service struct {
	Name      string
	Path      string
	Command   string
	Port      int
	AutoStart bool
}

// Result contains all detected project information.
type Result struct {
	ProjectName     string
	Services        []Service
	SuggestedDomain string
}

// inspector interface - each language/framework implements this.
type inspector interface {
	name() string
	inspect(root string) ([]Service, error)
}

// registry of all inspectors
var inspectors = []inspector{
	&nodeInspector{},
}

// Inspect scans a project directory and returns detected services.
func Inspect(root string) (*Result, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	result := &Result{
		ProjectName:     filepath.Base(absRoot),
		SuggestedDomain: filepath.Base(absRoot) + ".dev",
	}

	for _, i := range inspectors {
		services, err := i.inspect(absRoot)
		if err != nil {
			continue
		}
		result.Services = append(result.Services, services...)
	}

	return result, nil
}
