// Package etcd allows to create and manage etcd clusters.
package etcd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"sigs.k8s.io/yaml"

	"github.com/flexkube/libflexkube/internal/util"
	"github.com/flexkube/libflexkube/pkg/container"
	containertypes "github.com/flexkube/libflexkube/pkg/container/types"
	"github.com/flexkube/libflexkube/pkg/defaults"
	"github.com/flexkube/libflexkube/pkg/host"
	"github.com/flexkube/libflexkube/pkg/host/transport/ssh"
	"github.com/flexkube/libflexkube/pkg/pki"
	"github.com/flexkube/libflexkube/pkg/types"
)

// defaultDialTimeout is default timeout value for etcd client.
const defaultDialTimeout = 5 * time.Second

// Cluster represents etcd cluster configuration and state from the user.
//
// It implements types.ResourceConfig interface and via types.Resource interface
// allows to manage full lifecycle management of etcd cluster, including adding and
// removing members.
type Cluster struct {
	// Image allows to set Docker image with tag, which will be used by all members,
	// if members has no image set. If empty, etcd image defined in pkg/defaults
	// will be used.
	//
	// Example value: 'quay.io/coreos/etcd:v3.4.9'
	//
	// This field is optional.
	Image string `json:"image,omitempty"`

	// SSH stores common SSH configuration for all members and will be merged with members
	// SSH configuration. If member has some SSH fields defined, they take precedence over
	// this block.
	//
	// If you use same username or port for all members, it is recommended to have it defined
	// here to avoid repetition in the configuration.
	//
	// This field is optional.
	SSH *ssh.Config `json:"ssh,omitempty"`

	// CACertificate should contain etcd CA X.509 certificate in PEM format. It will be added
	// to members configuration if they don't have it defined.
	//
	// If empty, content will be pulled from PKI struct. The content can be also generated by the
	// pki.PKI object.
	//
	// This field is optional.
	CACertificate string `json:"caCertificate,omitempty"`

	// PeerCertAllowedCN defines allowed CommonName of the client certificate
	// for peer communication. Can be used when single client certificate is used
	// for all members of the cluster.
	//
	// Is is used for --peer-cert-allowed-cn flag.
	//
	// Example value: 'member'.
	//
	// This field is optional.
	PeerCertAllowedCN string `json:"peerCertAllowedCN,omitempty"`

	// Members is a list of etcd member containers to create, where key defines the member name.
	// Member name can be overwritten by setting Name field.
	//
	// If there is no state defined, this list must not be empty.
	//
	// If state is defined and list of members is empty, all created containers will be removed.
	Members map[string]MemberConfig `json:"members,omitempty"`

	// PKI field allows to use PKI resource for managing all etcd certificates. It will be used for
	// members configuration, if they don't have certificates defined.
	PKI *pki.PKI `json:"pki,omitempty"`

	// State stores state of the created containers. After deployment, it is up to the user to export
	// the state and restore it on consecutive runs.
	State container.ContainersState `json:"state,omitempty"`

	// ExtraMounts defines extra mounts from host filesystem, which should be added to member
	// containers. It will be used unless member define it's own extra mounts.
	ExtraMounts []containertypes.Mount `json:"extraMounts,omitempty"`
}

// cluster is executable version of Cluster, with validated fields and calculated containers.
type cluster struct {
	containers container.ContainersInterface
	members    map[string]Member
}

// propagateMember fills given Member's empty fields with fields from Cluster.
func (c *Cluster) propagateMember(i string, m *MemberConfig) {
	initialClusterArr := []string{}
	peerCertAllowedCNArr := []string{}

	for n, m := range c.Members {
		// If member has no name defined explicitly, use key passed as argument.
		name := util.PickString(m.Name, n)

		initialClusterArr = append(initialClusterArr, fmt.Sprintf("%s=https://%s:2380", name, m.PeerAddress))
		peerCertAllowedCNArr = append(peerCertAllowedCNArr, name)
	}

	sort.Strings(initialClusterArr)
	sort.Strings(peerCertAllowedCNArr)

	m.Name = util.PickString(m.Name, i)
	m.Image = util.PickString(m.Image, c.Image, defaults.EtcdImage)
	m.InitialCluster = util.PickString(m.InitialCluster, strings.Join(initialClusterArr, ","))
	m.PeerCertAllowedCN = util.PickString(m.PeerCertAllowedCN, c.PeerCertAllowedCN)
	m.CACertificate = util.PickString(m.CACertificate, c.CACertificate)

	if len(m.ExtraMounts) == 0 {
		m.ExtraMounts = c.ExtraMounts
	}

	// PKI integration.
	if c.PKI != nil && c.PKI.Etcd != nil {
		e := c.PKI.Etcd

		m.CACertificate = util.PickString(m.CACertificate, c.CACertificate, string(e.CA.X509Certificate))

		if c, ok := e.PeerCertificates[m.Name]; ok {
			m.PeerCertificate = util.PickString(m.PeerCertificate, string(c.X509Certificate))
			m.PeerKey = util.PickString(m.PeerKey, string(c.PrivateKey))
		}

		if c, ok := e.ServerCertificates[m.Name]; ok {
			m.ServerCertificate = util.PickString(m.ServerCertificate, string(c.X509Certificate))
			m.ServerKey = util.PickString(m.ServerKey, string(c.PrivateKey))
		}
	}

	m.ServerAddress = util.PickString(m.ServerAddress, m.PeerAddress)

	m.Host = host.BuildConfig(m.Host, host.Host{
		SSHConfig: c.SSH,
	})

	if len(c.State) == 0 {
		m.NewCluster = true
	}
}

// New validates etcd cluster configuration and fills members with default and computed values.
func (c *Cluster) New() (types.Resource, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validating cluster configuration: %w", err)
	}

	cc := container.Containers{
		PreviousState: c.State,
		DesiredState:  container.ContainersState{},
	}

	cluster := &cluster{
		members: map[string]Member{},
	}

	for n, m := range c.Members {
		m := m
		c.propagateMember(n, &m)

		mem, _ := m.New()                         //nolint:errcheck // We check it in Validate().
		hcc, _ := mem.ToHostConfiguredContainer() //nolint:errcheck // We check it in Validate().

		cc.DesiredState[n] = hcc

		cluster.members[n] = mem
	}

	co, _ := cc.New() //nolint:errcheck // We check it in Validate().

	cluster.containers = co

	return cluster, nil
}

// Validate validates Cluster configuration.
func (c *Cluster) Validate() error {
	if len(c.Members) == 0 && len(c.State) == 0 {
		return fmt.Errorf("at least one member must be defined when state is empty")
	}

	var errors util.ValidateErrors

	if c.CACertificate != "" {
		caCert := &pki.Certificate{
			X509Certificate: types.Certificate(c.CACertificate),
		}

		if _, err := caCert.DecodeX509Certificate(); err != nil {
			errors = append(errors, fmt.Errorf("parsing CA certificate: %w", err))
		}
	}

	cc := container.Containers{
		PreviousState: c.State,
		DesiredState:  container.ContainersState{},
	}

	for n, m := range c.Members {
		m := m
		c.propagateMember(n, &m)

		mem, err := m.New()
		if err != nil {
			errors = append(errors, fmt.Errorf("validating member %q: %w", n, err))

			continue
		}

		hcc, err := mem.ToHostConfiguredContainer()
		if err != nil {
			errors = append(errors, fmt.Errorf("validating member %q container: %w", n, err))

			continue
		}

		cc.DesiredState[n] = hcc
	}

	if _, err := cc.New(); err != nil {
		errors = append(errors, fmt.Errorf("validating containers object: %w", err))
	}

	return errors.Return()
}

// FromYaml allows to create and validate resource from YAML format.
func FromYaml(c []byte) (types.Resource, error) {
	return types.ResourceFromYaml(c, &Cluster{})
}

// StateToYaml allows to dump cluster state to YAML, so it can be restored later.
func (c *cluster) StateToYaml() ([]byte, error) {
	return yaml.Marshal(Cluster{State: c.containers.ToExported().PreviousState})
}

// CheckCurrentState refreshes current state of the cluster.
func (c *cluster) CheckCurrentState() error {
	if err := c.containers.CheckCurrentState(); err != nil {
		return fmt.Errorf("checking current state of etcd cluster: %w", err)
	}

	return nil
}

// getExistingEndpoints returns list of already deployed etcd endpoints.
func (c *cluster) getExistingEndpoints() []string {
	endpoints := []string{}

	for i, m := range c.members {
		if _, ok := c.containers.ToExported().PreviousState[i]; !ok {
			continue
		}

		endpoints = append(endpoints, fmt.Sprintf("%s:2379", m.peerAddress()))
	}

	return endpoints
}

func (c *cluster) firstMember() (Member, error) {
	for i := range c.members {
		return c.members[i], nil
	}

	return nil, fmt.Errorf("no members defined")
}

func (c *cluster) getClient() (etcdClient, error) {
	m, err := c.firstMember()
	if err != nil {
		return nil, fmt.Errorf("getting member object: %w", err)
	}

	endpoints, err := m.forwardEndpoints(c.getExistingEndpoints())
	if err != nil {
		return nil, fmt.Errorf("forwarding endpoints: %w", err)
	}

	return m.getEtcdClient(endpoints)
}

type etcdClient interface {
	MemberList(context context.Context) (*clientv3.MemberListResponse, error)
	MemberAdd(context context.Context, peerURLs []string) (*clientv3.MemberAddResponse, error)
	MemberRemove(context context.Context, id uint64) (*clientv3.MemberRemoveResponse, error)
	Close() error
}

func (c *cluster) membersToRemove() []string {
	m := []string{}

	e := c.containers.ToExported()

	for i := range e.PreviousState {
		if _, ok := e.DesiredState[i]; !ok {
			m = append(m, i)
		}
	}

	return m
}

func (c *cluster) membersToAdd() []string {
	m := []string{}

	e := c.containers.ToExported()

	for i := range e.DesiredState {
		if _, ok := e.PreviousState[i]; !ok {
			m = append(m, i)
		}
	}

	return m
}

// updateMembers adds and remove members from the cluster according to the configuration.
func (c *cluster) updateMembers(cli etcdClient) error {
	for _, name := range c.membersToRemove() {
		m := &member{
			config: &MemberConfig{
				Name: name,
			},
		}

		if err := m.remove(cli); err != nil {
			return fmt.Errorf("removing member: %w", err)
		}
	}

	for _, m := range c.membersToAdd() {
		if err := c.members[m].add(cli); err != nil {
			return fmt.Errorf("adding member: %w", err)
		}
	}

	return nil
}

// Deploy refreshes current state of the cluster and deploys detected changes.
func (c *cluster) Deploy() error {
	e := c.containers.ToExported()

	// If we create new cluster or destroy entire cluster, just start deploying.
	if len(e.PreviousState) != 0 && len(e.DesiredState) != 0 {
		// Build client, so we can pass it around.
		cli, err := c.getClient()
		if err != nil {
			return fmt.Errorf("getting etcd client: %w", err)
		}

		if err := c.updateMembers(cli); err != nil {
			return fmt.Errorf("updating members before deploying: %w", err)
		}

		if err := cli.Close(); err != nil {
			return fmt.Errorf("closing etcd client: %w", err)
		}
	}

	return c.containers.Deploy()
}

// Containers implement types.Resource interface.
func (c *cluster) Containers() container.ContainersInterface {
	return c.containers
}
