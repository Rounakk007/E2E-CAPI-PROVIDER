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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"golang.org/x/crypto/ssh"
)

const (
	// providerSSHKeySecretName is the name of the Secret that stores the provider's SSH key pair.
	providerSSHKeySecretName = "e2e-provider-ssh-key"
	// providerSSHKeyNamespace is the namespace where the SSH key secret is stored.
	providerSSHKeyNamespace = "default"
)

// SSHKeyPair holds the provider's SSH key pair.
type SSHKeyPair struct {
	PrivateKey []byte
	PublicKey  string
}

// EnsureSSHKeyPair ensures the provider's SSH key pair exists in a Kubernetes Secret.
// If the Secret doesn't exist, it generates a new key pair and creates the Secret.
func EnsureSSHKeyPair(ctx context.Context, c client.Client, namespace string) (*SSHKeyPair, error) {
	if namespace == "" {
		namespace = providerSSHKeyNamespace
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      providerSSHKeySecretName,
	}

	err := c.Get(ctx, key, secret)
	if err == nil {
		// Secret exists, read the key pair
		privateKey, ok := secret.Data["private-key"]
		if !ok {
			return nil, fmt.Errorf("SSH key secret %s/%s has no 'private-key' field", namespace, providerSSHKeySecretName)
		}
		publicKey, ok := secret.Data["public-key"]
		if !ok {
			return nil, fmt.Errorf("SSH key secret %s/%s has no 'public-key' field", namespace, providerSSHKeySecretName)
		}
		return &SSHKeyPair{
			PrivateKey: privateKey,
			PublicKey:  string(publicKey),
		}, nil
	}

	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("getting SSH key secret: %w", err)
	}

	// Generate a new key pair
	keyPair, err := generateSSHKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generating SSH key pair: %w", err)
	}

	// Create the Secret
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerSSHKeySecretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"private-key": keyPair.PrivateKey,
			"public-key":  []byte(keyPair.PublicKey),
		},
	}

	if err := c.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("creating SSH key secret: %w", err)
	}

	return keyPair, nil
}

// generateSSHKeyPair generates a new RSA SSH key pair.
func generateSSHKeyPair() (*SSHKeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("generating RSA key: %w", err)
	}

	// Encode private key to PEM
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Generate public key in OpenSSH format
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("generating SSH public key: %w", err)
	}
	publicKeyStr := string(ssh.MarshalAuthorizedKey(publicKey))

	return &SSHKeyPair{
		PrivateKey: privateKeyPEM,
		PublicKey:  publicKeyStr,
	}, nil
}
