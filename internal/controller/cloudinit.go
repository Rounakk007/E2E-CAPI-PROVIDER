/*
Copyright 2024 E2E Networks Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

// CloudInitConfig represents the relevant parts of a cloud-init config.
type CloudInitConfig struct {
	WriteFiles []WriteFile   `yaml:"write_files"`
	RunCmd     []interface{} `yaml:"runcmd"`
}

// WriteFile represents a single file entry in cloud-init write_files.
type WriteFile struct {
	Path        string `yaml:"path"`
	Owner       string `yaml:"owner"`
	Permissions string `yaml:"permissions"`
	Content     string `yaml:"content"`
}

// CloudInitToScript converts cloud-init YAML to a bash script that:
// 1. Writes all files from write_files
// 2. Executes all commands from runcmd
func CloudInitToScript(cloudInitData string) (string, error) {
	var config CloudInitConfig

	// The cloud-init data may start with #cloud-config, strip it
	data := strings.TrimPrefix(cloudInitData, "#cloud-config\n")

	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		return "", fmt.Errorf("parsing cloud-init YAML: %w", err)
	}

	var script strings.Builder
	script.WriteString("#!/bin/bash\nset -e\n\n")

	// Write files
	for _, f := range config.WriteFiles {
		if f.Path == "" || f.Content == "" {
			continue
		}
		// Create directory
		dir := f.Path[:strings.LastIndex(f.Path, "/")]
		script.WriteString(fmt.Sprintf("mkdir -p '%s'\n", dir))

		// Write content using heredoc
		script.WriteString(fmt.Sprintf("cat > '%s' << 'CAPI_EOF'\n%s\nCAPI_EOF\n", f.Path, f.Content))

		// Set permissions
		if f.Permissions != "" {
			script.WriteString(fmt.Sprintf("chmod %s '%s'\n", f.Permissions, f.Path))
		}

		// Set owner
		if f.Owner != "" {
			script.WriteString(fmt.Sprintf("chown %s '%s'\n", f.Owner, f.Path))
		}

		script.WriteString("\n")
	}

	// Run commands — each entry can be a string or a list of strings
	for _, cmd := range config.RunCmd {
		switch v := cmd.(type) {
		case string:
			script.WriteString(v + "\n")
		case []interface{}:
			parts := make([]string, 0, len(v))
			for _, p := range v {
				if s, ok := p.(string); ok {
					parts = append(parts, s)
				}
			}
			script.WriteString(strings.Join(parts, " ") + "\n")
		}
	}

	return script.String(), nil
}
