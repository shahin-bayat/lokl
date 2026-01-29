package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/shahin-bayat/lokl/internal/inspect"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new lokl.yaml from detected project structure",
	RunE:  runInit,
}

type serviceConfig struct {
	command   string
	port      int
	path      string
	subdomain string
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	if _, err := os.Stat(defaultConfigFile); err == nil {
		fmt.Printf("lokl.yaml already exists. Overwrite? [y/N] ")
		if !promptYesNo(false) {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Println("Scanning project...")
	result, err := inspect.Inspect(cwd)
	if err != nil {
		return fmt.Errorf("inspecting project: %w", err)
	}

	if len(result.Services) == 0 {
		fmt.Println("\nNo services detected.")
		fmt.Println("Create lokl.yaml manually or ensure your project has package.json with scripts.")
		return nil
	}

	fmt.Printf("\nDetected %d service(s):\n", len(result.Services))
	for _, svc := range result.Services {
		fmt.Printf("  - %s (%s)\n", svc.Name, svc.Path)
	}

	scanner := bufio.NewScanner(os.Stdin)
	services := make(map[string]serviceConfig)

	for _, svc := range result.Services {
		sc, ok := configureService(svc, cwd, scanner)
		if ok {
			services[svc.Name] = sc
		}
	}

	if len(services) == 0 {
		fmt.Println("\nNo services configured. Aborting.")
		return nil
	}

	fmt.Printf("\nBase domain [%s]: ", result.SuggestedDomain)
	domain := result.SuggestedDomain
	if scanner.Scan() {
		if text := strings.TrimSpace(scanner.Text()); text != "" {
			domain = text
		}
	}

	return saveConfig(result.ProjectName, domain, services)
}

func configureService(svc inspect.Service, cwd string, scanner *bufio.Scanner) (serviceConfig, bool) {
	fmt.Printf("\n─── %s ───\n", svc.Name)

	svcPath := svc.Path
	if svcPath == "." {
		svcPath = cwd
	} else {
		svcPath = filepath.Join(cwd, svc.Path)
	}

	scripts, err := inspect.GetScripts(svcPath)
	if err != nil {
		fmt.Printf("  Warning: could not read scripts: %v\n", err)
		return serviceConfig{}, false
	}

	scriptNames := inspect.SortScriptsByPriority(scripts)

	fmt.Println("  Available scripts:")
	for i, name := range scriptNames {
		fmt.Printf("    [%d] %s\n", i+1, name)
	}
	fmt.Printf("    [0] Skip this service\n")

	fmt.Printf("  Select script [1]: ")
	choice := 1
	if scanner.Scan() {
		if n, err := strconv.Atoi(strings.TrimSpace(scanner.Text())); err == nil {
			choice = n
		}
	}

	if choice == 0 || choice > len(scriptNames) {
		fmt.Printf("  Skipping %s\n", svc.Name)
		return serviceConfig{}, false
	}

	selectedScript := scriptNames[choice-1]
	scriptCmd := scripts[selectedScript]

	port := inspect.InferPort(scriptCmd)
	if port == 0 {
		fmt.Printf("  Port (required): ")
		if scanner.Scan() {
			if n, err := strconv.Atoi(strings.TrimSpace(scanner.Text())); err == nil {
				port = n
			}
		}
	} else {
		fmt.Printf("  Port [%d]: ", port)
		if scanner.Scan() {
			if text := strings.TrimSpace(scanner.Text()); text != "" {
				if n, err := strconv.Atoi(text); err == nil {
					port = n
				}
			}
		}
	}

	if port == 0 {
		fmt.Printf("  Skipping %s (no port)\n", svc.Name)
		return serviceConfig{}, false
	}

	subdomain := svc.Name
	fmt.Printf("  Subdomain [%s]: ", subdomain)
	if scanner.Scan() {
		if text := strings.TrimSpace(scanner.Text()); text != "" {
			subdomain = text
		}
	}

	sc := serviceConfig{
		command:   svc.Command + " run " + selectedScript,
		port:      port,
		subdomain: subdomain,
	}
	if svc.Path != "." {
		sc.path = svc.Path
	}

	return sc, true
}

func saveConfig(projectName, domain string, services map[string]serviceConfig) error {
	svcMap := make(map[string]map[string]any)
	for name, svc := range services {
		m := map[string]any{
			"command":   svc.command,
			"port":      svc.port,
			"subdomain": svc.subdomain,
		}
		if svc.path != "" {
			m["path"] = svc.path
		}
		svcMap[name] = m
	}

	cfg := map[string]any{
		"name":    projectName,
		"version": "1",
		"proxy": map[string]string{
			"domain": domain,
		},
		"services": svcMap,
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(defaultConfigFile, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("\n✓ Created %s\n", defaultConfigFile)
	fmt.Println("✓ Run 'lokl up' to start your environment")

	return nil
}

func promptYesNo(defaultYes bool) bool {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		text := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if text == "y" || text == "yes" {
			return true
		}
		if text == "n" || text == "no" {
			return false
		}
	}
	return defaultYes
}
