package controlplane

import (
	"encoding/base64"
	"fmt"

	"github.com/invidian/flexkube/pkg/container"
	"github.com/invidian/flexkube/pkg/container/runtime/docker"
	"github.com/invidian/flexkube/pkg/container/types"
	"github.com/invidian/flexkube/pkg/defaults"
	"github.com/invidian/flexkube/pkg/host"
)

type KubeScheduler struct {
	Image                   string     `json:"image,omitempty" yaml:"image,omitempty"`
	Host                    *host.Host `json:"host,omitempty" yaml:"host,omitempty"`
	KubernetesCACertificate string     `json:"kubernetesCACertificate,omitempty" yaml:"kubernetesCACertificate,omitempty"`
	APIServer               string     `json:"apiServer,omitempty" yaml:"apiServer,omitempty"`
	// TODO don't take the admin key, use dedicated certificate for static controller manager,
	// which will have a group + create a binding to system:kube-controller-manager clusterRole
	// as done in self-hosted chart.
	// TODO since we have access to CA cert and key, we could generate certificate ourselves here
	AdminCertificate string `json:"adminCertificate,omitempty" yaml"adminCertificate,omitempty"`
	AdminKey         string `json:"adminKey,omitempty" yaml:"adminKey,omitempty"`
}

type kubeScheduler struct {
	image                   string
	host                    host.Host
	kubernetesCACertificate string
	apiServer               string
	adminCertificate        string
	adminKey                string
}

func (k *kubeScheduler) ToHostConfiguredContainer() *container.HostConfiguredContainer {
	configFiles := make(map[string]string)
	// TODO put all those path in a single place. Perhaps make them configurable with defaults too
	configFiles["/etc/kubernetes/kube-scheduler/kubeconfig"] = k.toKubeconfig()

	c := container.Container{
		// TODO this is weird. This sets docker as default runtime config
		Runtime: container.RuntimeConfig{
			Docker: &docker.ClientConfig{},
		},
		Config: types.ContainerConfig{
			Name:        "kube-scheduler",
			Image:       k.image,
			Entrypoint:  []string{"/hyperkube"},
			NetworkMode: "host",
			Mounts: []types.Mount{
				types.Mount{
					Source: "/etc/kubernetes/kube-scheduler/kubeconfig",
					Target: "/etc/kubernetes/kubeconfig",
				},
			},
			Args: []string{
				"kube-scheduler",
				"--kubeconfig=/etc/kubernetes/kubeconfig",
			},
		},
	}

	return &container.HostConfiguredContainer{
		Host:        k.host,
		ConfigFiles: configFiles,
		Container:   c,
	}
}

func (k *KubeScheduler) New() (*kubeScheduler, error) {
	if err := k.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate Kubernetes Scheduler configuration: %w", err)
	}

	nk := &kubeScheduler{
		image:                   k.Image,
		host:                    *k.Host,
		kubernetesCACertificate: k.KubernetesCACertificate,
		apiServer:               k.APIServer,
		adminCertificate:        k.AdminCertificate,
		adminKey:                k.AdminKey,
	}

	// The only optional parameter
	if nk.image == "" {
		nk.image = defaults.KubernetesImage
	}

	return nk, nil
}

// TODO add validation of certificates if specified
func (k *KubeScheduler) Validate() error {
	if k.KubernetesCACertificate == "" {
		return fmt.Errorf("KubernetesCACertificate is empty")
	}
	if k.APIServer == "" {
		return fmt.Errorf("APIServer is empty")
	}
	if k.AdminCertificate == "" {
		return fmt.Errorf("AdminCertificate is empty")
	}
	if k.AdminKey == "" {
		return fmt.Errorf("AdminKey is empty")
	}

	return nil
}

// TODO this is quite generic, refactor it
func (k *kubeScheduler) toKubeconfig() string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: static
  cluster:
    server: https://%s:6443
    certificate-authority-data: %s
users:
- name: static
  user:
    client-certificate-data: %s
    client-key-data: %s
current-context: static
contexts:
- name: static
  context:
    cluster: static
    user: static
`, k.apiServer, base64.StdEncoding.EncodeToString([]byte(k.kubernetesCACertificate)), base64.StdEncoding.EncodeToString([]byte(k.adminCertificate)), base64.StdEncoding.EncodeToString([]byte(k.adminKey)))
}