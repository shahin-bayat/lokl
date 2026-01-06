package config

import (
	"fmt"
	"sort"
)

// SortByDependency returns service names in start order using topological sort.
// Services with no dependencies come first, then their dependents.
func SortByDependency(services map[string]Service) ([]string, error) {
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

	// Start with services that have no dependencies (sorted for stable order)
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		result = append(result, name)

		// Collect ready dependents and sort for stable order
		var ready []string
		for _, dependent := range dependents[name] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		sort.Strings(ready)
		queue = append(queue, ready...)
	}

	if len(result) != len(services) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return result, nil
}
