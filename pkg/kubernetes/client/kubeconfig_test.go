package client_test

import (
	"fmt"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/flexkube/libflexkube/internal/util"
	"github.com/flexkube/libflexkube/internal/utiltest"
	"github.com/flexkube/libflexkube/pkg/kubernetes/client"
	"github.com/flexkube/libflexkube/pkg/types"
)

// GetKubeconfig returns content of fake kubeconfig file for testing.
func GetKubeconfig(t *testing.T) string {
	t.Helper()

	pki := utiltest.GeneratePKI(t)

	y := fmt.Sprintf(`server: %s
caCertificate: |
  %s
clientCertificate: |
  %s
clientKey: |
  %s
`,
		"localhost",
		strings.TrimSpace(util.Indent(pki.Certificate, "  ")),
		strings.TrimSpace(util.Indent(pki.Certificate, "  ")),
		strings.TrimSpace(util.Indent(pki.PrivateKey, "  ")),
	)

	c := &client.Config{}

	if err := yaml.Unmarshal([]byte(y), c); err != nil {
		t.Fatalf("unmarshaling config should succeed, got: %v", err)
	}

	kubeconfig, err := c.ToYAMLString()
	if err != nil {
		t.Fatalf("Generating kubeconfig should work, got: %v", err)
	}

	return kubeconfig
}

// ToYAMLString() tests.
func TestUnmarshal(t *testing.T) {
	t.Parallel()

	if kubeconfig := GetKubeconfig(t); kubeconfig == "" {
		t.Fatalf("Generated kubeconfig shouldn't be empty")
	}
}

func TestToYAMLStringNew(t *testing.T) { //nolint:funlen
	t.Parallel()

	cases := []struct {
		f   func(*client.Config)
		err func(*testing.T, error)
	}{
		{
			func(c *client.Config) {
				c.CACertificate = "ddd"
			},
			func(t *testing.T, err error) { //nolint:thelper
				if err == nil {
					t.Errorf("Kubeconfig with bad CA Certificate should be invalid")
				}
			},
		},
		{
			func(c *client.Config) {
				c.ClientCertificate = "dfoo"
			},
			func(t *testing.T, err error) { //nolint:thelper
				if err == nil {
					t.Errorf("Kubeconfig with bad client certificate should be invalid")
				}
			},
		},
		{
			func(c *client.Config) {
				c.ClientKey = "ffoo"
			},
			func(t *testing.T, err error) { //nolint:thelper
				if err == nil {
					t.Errorf("Kubeconfig with bad client key should be invalid")
				}
			},
		},
		{
			func(c *client.Config) {
				pki := utiltest.GeneratePKI(t)
				c.ClientKey = types.PrivateKey(pki.PrivateKey)
			},
			func(t *testing.T, err error) { //nolint:thelper
				if err == nil {
					t.Errorf("Kubeconfig with not matching client key should be invalid")
				}
			},
		},
		{
			func(c *client.Config) {},
			func(t *testing.T, err error) { //nolint:thelper
				if err != nil {
					t.Errorf("Valid config shouldn't return error, got: %v", err)
				}
			},
		},
		{
			func(c *client.Config) {
				c.ClientCertificate = ""
				c.ClientKey = ""
				c.Token = "doo"
			},
			func(t *testing.T, err error) { //nolint:thelper
				if err != nil {
					t.Errorf("config with only token set should be valid, got: %v", err)
				}
			},
		},
		{
			func(c *client.Config) {
				c.ClientCertificate = ""
				c.Token = "roo"
			},
			func(t *testing.T, err error) { //nolint:thelper
				if err == nil {
					t.Errorf("config with token and client key set should not be valid")
				}
			},
		},
		{
			func(c *client.Config) {
				c.ClientKey = ""
				c.Token = "fnoo"
			},
			func(t *testing.T, err error) { //nolint:thelper
				if err == nil {
					t.Errorf("config with token and client certificate set should be valid")
				}
			},
		},
	}

	for n, c := range cases {
		c := c

		t.Run(fmt.Sprintf("%d", n), func(t *testing.T) {
			t.Parallel()

			pki := utiltest.GeneratePKI(t)

			config := &client.Config{
				Server:            "localhost",
				CACertificate:     types.Certificate(pki.Certificate),
				ClientCertificate: types.Certificate(pki.Certificate),
				ClientKey:         types.PrivateKey(pki.PrivateKey),
			}

			c.f(config)

			_, err := config.ToYAMLString()

			c.err(t, err)
		})
	}
}

func TestToYAMLStringValidate(t *testing.T) {
	t.Parallel()

	pki := utiltest.GeneratePKI(t)

	c := &client.Config{
		CACertificate:     types.Certificate(pki.Certificate),
		ClientCertificate: types.Certificate(pki.Certificate),
		ClientKey:         types.PrivateKey(pki.PrivateKey),
	}

	if _, err := c.ToYAMLString(); err == nil {
		t.Fatalf("ToYAMLString should validate the configuration")
	}
}

// Validate() tests.
func TestValidate(t *testing.T) { //nolint:funlen
	t.Parallel()

	cases := []struct {
		f   func(*client.Config)
		err func(*testing.T, error)
	}{
		{
			func(c *client.Config) {
				c.Server = ""
			},
			func(t *testing.T, err error) { //nolint:thelper
				if err == nil {
					t.Errorf("Kubeconfig without defined server should be invalid")
				}
			},
		},
		{
			func(c *client.Config) {
				c.CACertificate = ""
			},
			func(t *testing.T, err error) { //nolint:thelper
				if err == nil {
					t.Errorf("Kubeconfig without defined CA Certificate should be invalid")
				}
			},
		},
		{
			func(c *client.Config) {
				c.ClientCertificate = ""
			},
			func(t *testing.T, err error) { //nolint:thelper
				if err == nil {
					t.Errorf("Kubeconfig without defined client certificate should be invalid")
				}
			},
		},
		{
			func(c *client.Config) {
				c.ClientKey = ""
			},
			func(t *testing.T, err error) { //nolint:thelper
				if err == nil {
					t.Errorf("Kubeconfig without defined client key should be invalid")
				}
			},
		},

		{
			func(c *client.Config) {},
			func(t *testing.T, err error) { //nolint:thelper
				if err != nil {
					t.Errorf("Valid config shouldn't return error, got: %v", err)
				}
			},
		},
	}

	for n, c := range cases {
		c := c

		t.Run(fmt.Sprintf("%d", n), func(t *testing.T) {
			t.Parallel()

			pki := utiltest.GeneratePKI(t)

			config := &client.Config{
				Server:            "localhost",
				CACertificate:     types.Certificate(pki.Certificate),
				ClientCertificate: types.Certificate(pki.Certificate),
				ClientKey:         types.PrivateKey(pki.PrivateKey),
			}

			c.f(config)

			c.err(t, config.Validate())
		})
	}
}
