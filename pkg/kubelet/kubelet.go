package kubelet

import (
	"fmt"

	"github.com/invidian/flexkube/pkg/container"
	"github.com/invidian/flexkube/pkg/container/runtime/docker"
	"github.com/invidian/flexkube/pkg/container/types"
	"github.com/invidian/flexkube/pkg/defaults"
	"github.com/invidian/flexkube/pkg/host"
)

// Instance represents single kubelet instance
type Kubelet struct {
	Address             string     `json:"address,omitempty" yaml:"address,omitempty"`
	Image               string     `json:"image,omitempty" yaml:"image,omitempty"`
	Host                *host.Host `json:"host,omitempty" yaml:"host,omitempty"`
	BootstrapKubeconfig string     `json:"bootstrapKubeconfig,omitempty" yaml:"bootstrapKubeconfig,omitempty"`
	// TODO we requre CA certificate, so it can be referred in bootstrap-kubeconfig. Maybe we should be responsible for creating
	// bootstrap-kubeconfig too then?
	KubernetesCACertificate string `json:"kubernetesCACertificate,omitempty" yaml"kubernetesCACertificate,omitempty"`

	// Depending on the network plugin, this should be optional, but for now it's required.
	PodCIDR string `json:"podCIDR,omitempty" yaml:"podCIDR,omitempty"`
}

// kubelet is a validated, executable version of Kubelet
type kubelet struct {
	address                 string
	image                   string
	host                    *host.Host
	bootstrapKubeconfig     string
	kubernetesCACertificate string
	podCIDR                 string
}

func (k *Kubelet) New() (*kubelet, error) {
	// TODO when creating kubelet, also pull pause image using configured Container Runtime to speed up later start of pods?
	if err := k.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate kubelet configuration: %w", err)
	}

	nk := &kubelet{
		image:                   k.Image,
		address:                 k.Address,
		host:                    k.Host,
		bootstrapKubeconfig:     k.BootstrapKubeconfig,
		kubernetesCACertificate: k.KubernetesCACertificate,
		podCIDR:                 k.PodCIDR,
	}

	if nk.image == "" {
		nk.image = defaults.KubernetesImage
	}

	return nk, nil
}

func (k *Kubelet) Validate() error {
	// TODO better validation should be done here
	if k.BootstrapKubeconfig == "" {
		return fmt.Errorf("bootstrapKubeconfig can't be empty")
	}

	return nil
}

func (k *kubelet) ToHostConfiguredContainer() *container.HostConfiguredContainer {
	configFiles := make(map[string]string)
	// TODO maybe we store that as a struct, and we marshal here to YAML?
	configFiles["/etc/kubernetes/kubelet/kubelet.yaml"] = fmt.Sprintf(`apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
rotateCertificates: true
cgroupsPerQOS: false
enforceNodeAllocatable: []
cgroupDriver: systemd
podCIDR: %s
address: %s
# Disable healht port for now, since we don't use it
# TODO check how to use it and re-enable it
healthzPort: 0
`, k.podCIDR, k.address)

	configFiles["/etc/kubernetes/kubelet/bootstrap-kubeconfig"] = k.bootstrapKubeconfig
	configFiles["/etc/kubernetes/kubelet/pki/ca.crt"] = k.kubernetesCACertificate

	c := container.Container{
		// TODO this is weird. This sets docker as default runtime config
		Runtime: container.RuntimeConfig{
			Docker: &docker.ClientConfig{},
		},
		Config: types.ContainerConfig{
			// TODO make it configurable?
			Name:  "kubelet",
			Image: k.image,
			// TODO perhaps entrypoint should be a string, not array of strings? we use args for arguments anyway
			Entrypoint: []string{"/kubelet"},
			Ports: []types.PortMap{
				types.PortMap{
					IP:       k.address,
					Protocol: "tcp",
					Port:     10250,
				},
			},
			Privileged: true,
			// Required for detecting node IP address
			NetworkMode: "host",
			// Required for adding containers into correct network namespaces
			PidMode: "host",
			Mounts: []types.Mount{
				types.Mount{
					Source: "/etc/kubernetes/kubelet/",
					Target: "/etc/kubernetes/",
				},
				types.Mount{
					Source: "/run/docker.sock",
					Target: "/var/run/docker.sock",
				},
				// For testing kubenet
				types.Mount{
					Source: "/etc/cni/net.d/",
					Target: "/etc/cni/net.d",
				},
				types.Mount{
					Source: "/opt/cni/bin/",
					Target: "/opt/cni/bin",
				},
				types.Mount{
					Source:      "/var/lib/docker/",
					Target:      "/var/lib/docker",
					Propagation: "rslave",
				},
				types.Mount{
					Source:      "/var/lib/kubelet/",
					Target:      "/var/lib/kubelet",
					Propagation: "shared",
				},
				types.Mount{
					Source: "/var/lib/cni/",
					Target: "/var/lib/cni",
				},
				types.Mount{
					// For loading kernel modules for kubenet plugin
					Source: "/lib/modules/",
					Target: "/lib/modules",
				},
			},
			Args: []string{
				"--config=/etc/kubernetes/kubelet.yaml",
				"--kubeconfig=/etc/kubernetes/kubeconfig",
				"--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubeconfig",
				"--v=2",
				"--network-plugin=kubenet",
			},
		},
	}

	return &container.HostConfiguredContainer{
		Host:        *k.host,
		ConfigFiles: configFiles,
		Container:   c,
	}
}
