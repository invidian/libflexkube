// TODO figure out better name for this package, maybe something more generic?
package apiloadbalancer

import (
	"fmt"

	"github.com/invidian/flexkube/pkg/container"
	"github.com/invidian/flexkube/pkg/container/runtime/docker"
	"github.com/invidian/flexkube/pkg/container/types"
	"github.com/invidian/flexkube/pkg/defaults"
	"github.com/invidian/flexkube/pkg/host"
)

type APILoadBalancer struct {
	Image              string     `json:"image,omitempty" yaml:"image,omitempty"`
	Host               *host.Host `json:"host,omitempty" yaml:"host,omitempty"`
	MetricsBindAddress string     `json:"metricsBindAddress,omitempty" yaml:"metricsBindAddress,omitempty"`
	// TODO should perhaps be int
	MetricsBindPort string   `json:"metricsBindPort,omitempty" yaml:"metricsBindPort,omitempty"`
	Servers         []string `json:"servers,omitempty" yaml:"servers,omitempty"`
}

type apiLoadBalancer struct {
	image              string
	host               *host.Host
	servers            []string
	metricsBindAddress string
	metricsBindPort    string
}

// TODO ToHostConfiguredContainer should become an interface, since we use this pattern in all packages
func (a *apiLoadBalancer) ToHostConfiguredContainer() *container.HostConfiguredContainer {
	servers := ""
	for i, s := range a.servers {
		servers = fmt.Sprintf("%s	server %d %s:8443 check\n	server %d-k8s %s:30443 check\n", servers, i, s, i, s)
	}

	configFiles := make(map[string]string)
	configFiles["/etc/haproxy/haproxy.cfg"] = fmt.Sprintf(`defaults
	# Do TLS passtrough
	mode tcp
	# Required values for both frontend and backend
	timeout connect 5000ms
	timeout client 50000ms
	timeout server 50000ms

frontend kube-apiserver
	# TODO make it configurable
	bind 0.0.0.0:6443
	default_backend kube-apiserver

backend kube-apiserver
	%s

frontend stats
	bind %s:%s
	mode http
	option http-use-htx
	http-request use-service prometheus-exporter if { path /metrics }
	stats enable
	stats uri /stats
	stats refresh 10s
`, servers, a.metricsBindAddress, a.metricsBindPort)

	c := container.Container{
		// TODO this is weird. This sets docker as default runtime config
		Runtime: container.RuntimeConfig{
			Docker: &docker.ClientConfig{},
		},
		Config: types.ContainerConfig{
			// TODO make it configurable? And don't force user to use HAProxy
			Name:  "api-loadbalancer-haproxy",
			Image: a.image,
			// TODO perhaps entrypoint should be a string, not array of strings? we use args for arguments anyway
			NetworkMode: "host",
			Mounts: []types.Mount{
				types.Mount{
					Source: "/etc/haproxy/haproxy.cfg",
					Target: "/usr/local/etc/haproxy/haproxy.cfg",
				},
			},
		},
	}

	return &container.HostConfiguredContainer{
		Host:        *a.host,
		ConfigFiles: configFiles,
		Container:   c,
	}

	return nil
}

func (a *APILoadBalancer) New() (*apiLoadBalancer, error) {
	if err := a.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate API Load balancer configuration: %w", err)
	}

	na := &apiLoadBalancer{
		image:              a.Image,
		host:               a.Host,
		servers:            a.Servers,
		metricsBindAddress: a.MetricsBindAddress,
		metricsBindPort:    a.MetricsBindPort,
	}

	// Fill empty fields with default values
	if na.image == "" {
		na.image = defaults.HAProxyImage
	}
	if na.metricsBindPort == "" {
		na.metricsBindPort = "8080"
	}

	return na, nil
}

func (a *APILoadBalancer) Validate() error {
	if a.Host == nil {
		return fmt.Errorf("Host must be set")
	}
	if len(a.Servers) <= 0 {
		return fmt.Errorf("At least one server must be set")
	}
	if a.MetricsBindAddress == "" {
		return fmt.Errorf("MetricsBindAddress must be set")
	}

	return nil
}
