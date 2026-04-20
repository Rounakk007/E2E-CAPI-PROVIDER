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

package cloud

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient provides SSH access to E2E compute nodes.
type SSHClient struct {
	privateKey []byte
}

// NewSSHClient creates a new SSH client with the given private key.
func NewSSHClient(privateKey []byte) *SSHClient {
	return &SSHClient{privateKey: privateKey}
}

// RunCommand connects to the host via SSH and executes the command as root.
func (s *SSHClient) RunCommand(host string, port int, user string, command string) (string, error) {
	signer, err := ssh.ParsePrivateKey(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("parsing SSH private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("SSH dial to %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("executing command via SSH: %w (output: %s)", err, string(output))
	}

	return string(output), nil
}

// IsSSHReady checks if the host is accepting SSH connections.
func (s *SSHClient) IsSSHReady(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
