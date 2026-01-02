package process

import (
	"fmt"

	"github.com/shahin-bayat/lokl/internal/config"
)

// SortByDependency returns service names in start order using topological sort.
// Services with no dependencies come first, then their dependents.
func SortByDependency(services map[string]config.Service) ([]string, error) {
	// inDegree: how many dependencies each service has
	inDegree := make(map[string]int)
	// dependents: who depends on this service
	dependents := make(map[string][]string)

	for name := range services {
		inDegree[name] = 0
	}

	for name, svc := range services {
		for _, dep := range svc.DependsOn {
			if _, exists := services[dep]; !exists {
				return nil, fmt.Errorf("service %q depends on unknown service %q", name, dep)
			}
			inDegree[name]++
			dependents[dep] = append(dependents[dep], name)
		}
	}

	// Start with services that have no dependencies
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	var result []string
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		result = append(result, name)

		// Decrement in-degree for dependents, add to queue if ready
		for _, dependent := range dependents[name] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(result) != len(services) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return result, nil
}
