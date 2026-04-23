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

// kubePreinstallScript installs containerd, kubelet, and kubeadm on Ubuntu.
// This is needed because E2E's base Ubuntu image doesn't have Kubernetes components.
const kubePreinstallScript = `
echo "=== Installing Kubernetes prerequisites ==="

# Skip if kubeadm is already installed
if command -v kubeadm &> /dev/null; then
    echo "kubeadm already installed, skipping prerequisites"
else
    # Stop unattended-upgrades and wait for dpkg lock — Ubuntu VMs run this at boot
    systemctl stop unattended-upgrades 2>/dev/null || true
    systemctl kill --kill-who=all unattended-upgrades 2>/dev/null || true
    while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 \
       || fuser /var/lib/apt/lists/lock >/dev/null 2>&1 \
       || fuser /var/lib/dpkg/lock >/dev/null 2>&1; do
        echo "Waiting for dpkg lock to be released..."
        sleep 5
    done

    # Disable swap
    swapoff -a
    sed -i '/swap/d' /etc/fstab

    # Load required kernel modules
    cat > /etc/modules-load.d/k8s.conf << EOF
overlay
br_netfilter
EOF
    modprobe overlay
    modprobe br_netfilter

    # Set sysctl params
    cat > /etc/sysctl.d/k8s.conf << EOF
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF
    sysctl --system

    # Install containerd
    apt-get update -q
    apt-get install -y -q apt-transport-https ca-certificates curl gnupg

    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc

    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list

    apt-get update -q
    apt-get install -y -q containerd.io

    # Configure containerd to use systemd cgroup
    mkdir -p /etc/containerd
    containerd config default > /etc/containerd/config.toml
    sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
    systemctl restart containerd
    systemctl enable containerd

    # Ensure containerd socket is available at the expected path
    mkdir -p /var/run/containerd
    if [ ! -S /var/run/containerd/containerd.sock ] && [ -S /run/containerd/containerd.sock ]; then
        ln -sf /run/containerd/containerd.sock /var/run/containerd/containerd.sock
    fi

    # Install kubeadm, kubelet, kubectl
    KUBE_VERSION="${KUBE_VERSION:-1.30}"
    curl -fsSL "https://pkgs.k8s.io/core:/stable:/v${KUBE_VERSION}/deb/Release.key" | gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
    echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v${KUBE_VERSION}/deb/ /" > /etc/apt/sources.list.d/kubernetes.list

    apt-get update -q
    apt-get install -y -q kubelet kubeadm kubectl
    apt-mark hold kubelet kubeadm kubectl

    systemctl enable kubelet

    echo "=== Kubernetes prerequisites installed ==="
fi

# Ensure containerd is running and socket is accessible
systemctl start containerd
mkdir -p /var/run/containerd
if [ ! -S /var/run/containerd/containerd.sock ] && [ -S /run/containerd/containerd.sock ]; then
    ln -sf /run/containerd/containerd.sock /var/run/containerd/containerd.sock
fi
`

// CloudInitToScript converts cloud-init YAML to a bash script that:
// 1. Sets the hostname to the machine name (prevents E2E hostname reuse conflicts)
// 2. Installs Kubernetes prerequisites (containerd, kubeadm, kubelet)
// 3. Writes all files from write_files
// 4. Executes all commands from runcmd
func CloudInitToScript(cloudInitData string, kubeVersion string, machineName string) (string, error) {
	var config CloudInitConfig

	// The cloud-init data may start with #cloud-config, strip it
	data := strings.TrimPrefix(cloudInitData, "#cloud-config\n")

	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		return "", fmt.Errorf("parsing cloud-init YAML: %w", err)
	}

	var script strings.Builder
	script.WriteString("#!/bin/bash\nset -e\nexport DEBIAN_FRONTEND=noninteractive\n\n")

	// Set hostname to the CAPI machine name so each node gets a unique,
	// stable hostname. This prevents E2E hostname reuse conflicts during
	// rolling upgrades where a new VM might get the same OS hostname as a
	// recently deleted node that still exists in the workload cluster.
	if machineName != "" {
		script.WriteString(fmt.Sprintf("hostnamectl set-hostname '%s'\n", machineName))
		script.WriteString(fmt.Sprintf("echo '127.0.0.1 %s' >> /etc/hosts\n\n", machineName))
	}

	// Set Kubernetes version for the preinstall script
	if kubeVersion != "" {
		// Extract major.minor from version like "v1.30.0" -> "1.30"
		ver := strings.TrimPrefix(kubeVersion, "v")
		parts := strings.SplitN(ver, ".", 3)
		if len(parts) >= 2 {
			script.WriteString(fmt.Sprintf("export KUBE_VERSION='%s.%s'\n\n", parts[0], parts[1]))
		}
	}

	// Install prerequisites
	script.WriteString(kubePreinstallScript)
	script.WriteString("\n")

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

	// Run commands — skip if bootstrap already completed
	script.WriteString("# Run bootstrap commands (skip if already done)\n")
	script.WriteString("if [ -f /run/cluster-api/bootstrap-success.complete ]; then\n")
	script.WriteString("    echo 'Bootstrap already completed, skipping runcmd'\n")
	script.WriteString("else\n")
	script.WriteString("    mkdir -p /run/cluster-api\n")
	script.WriteString("    # Reset any partial kubeadm state from previous attempts\n")
	script.WriteString("    if [ -f /etc/kubernetes/manifests/kube-apiserver.yaml ]; then\n")
	script.WriteString("        echo 'Cleaning up partial kubeadm state...'\n")
	script.WriteString("        kubeadm reset -f 2>/dev/null || true\n")
	script.WriteString("        rm -rf /var/lib/etcd/*\n")
	script.WriteString("        # Wait for the LB health check to propagate the new backend before\n")
	script.WriteString("        # retrying kubeadm init. Without this, kubeadm's upload-config phase\n")
	script.WriteString("        # hits the LB endpoint too quickly after the API server starts and\n")
	script.WriteString("        # gets EOF/timeout because the LB hasn't yet marked the backend healthy.\n")
	script.WriteString("        echo 'Waiting 90s for LB health check to propagate before retry...'\n")
	script.WriteString("        sleep 90\n")
	script.WriteString("        # Re-write certificate files after reset\n")
	for _, f := range config.WriteFiles {
		if f.Path == "" || f.Content == "" {
			continue
		}
		if strings.Contains(f.Path, "/etc/kubernetes/") {
			dir := f.Path[:strings.LastIndex(f.Path, "/")]
			script.WriteString(fmt.Sprintf("        mkdir -p '%s'\n", dir))
			script.WriteString(fmt.Sprintf("        cat > '%s' << 'CAPI_RESET_EOF'\n%s\nCAPI_RESET_EOF\n", f.Path, f.Content))
			if f.Permissions != "" {
				script.WriteString(fmt.Sprintf("        chmod %s '%s'\n", f.Permissions, f.Path))
			}
			if f.Owner != "" {
				script.WriteString(fmt.Sprintf("        chown %s '%s'\n", f.Owner, f.Path))
			}
		}
	}
	script.WriteString("    fi\n")
	for _, cmd := range config.RunCmd {
		var cmdStr string
		switch v := cmd.(type) {
		case string:
			cmdStr = v
		case []interface{}:
			parts := make([]string, 0, len(v))
			for _, p := range v {
				if s, ok := p.(string); ok {
					parts = append(parts, s)
				}
			}
			cmdStr = strings.Join(parts, " ")
		}

		// For kubeadm init, skip the upload-config phase so we can wait for the
		// LB health check to propagate after the API server starts. The LB marks
		// the backend unhealthy while the API server is down (during package
		// installation). Once the API server starts, the LB needs one health-check
		// cycle before it will forward traffic — running upload-config immediately
		// causes EOF. We sleep 60s after init, then run upload-config separately.
		if strings.Contains(cmdStr, "kubeadm init") {
			// Extract --config path (default is the CAPI kubeadm bootstrap path)
			configPath := "/run/kubeadm/kubeadm.yaml"
			if idx := strings.Index(cmdStr, "--config "); idx >= 0 {
				rest := cmdStr[idx+9:]
				if fields := strings.Fields(rest); len(fields) > 0 {
					configPath = fields[0]
				}
			}
			skipCmd := strings.Replace(cmdStr, "kubeadm init", "kubeadm init --skip-phases=upload-config", 1)
			script.WriteString("    " + skipCmd + "\n")
			script.WriteString("    echo 'API server started; waiting 60s for LB health check to mark backend healthy...'\n")
			script.WriteString("    sleep 60\n")
			script.WriteString(fmt.Sprintf("    kubeadm init phase upload-config all --config %s\n", configPath))
		} else {
			script.WriteString("    " + cmdStr + "\n")
		}
	}
	script.WriteString("fi\n")

	return script.String(), nil
}
