package direct_test

import (
	"testing"

	"github.com/flexkube/libflexkube/pkg/host/transport"
	"github.com/flexkube/libflexkube/pkg/host/transport/direct"
)

func newDirect(t *testing.T) transport.Interface {
	t.Helper()

	d := &direct.Config{}

	di, err := d.New()
	if err != nil {
		t.Fatalf("should return new object without errors, got: %v", err)
	}

	return di
}

func TestValidate(t *testing.T) {
	t.Parallel()

	d := &direct.Config{}

	if err := d.Validate(); err != nil {
		t.Fatalf("validation should always pass, got: %v", err)
	}
}

func TestForwardUnixSocket(t *testing.T) {
	t.Parallel()

	d := newDirect(t)
	p := "/foo" //nolint:ifshort

	dc, err := d.Connect()
	if err != nil {
		t.Fatalf("Connecting: %v", err)
	}

	if fp, _ := dc.ForwardUnixSocket(p); fp != p {
		t.Fatalf("expected '%s', got '%s'", p, fp)
	}
}

func TestConnect(t *testing.T) {
	t.Parallel()

	d := newDirect(t)

	if _, err := d.Connect(); err != nil {
		t.Fatalf("Connect should always work, got: %v", err)
	}
}

func TestForwardTCP(t *testing.T) {
	t.Parallel()

	d := newDirect(t)
	a := "localhost:80" //nolint:ifshort

	dc, err := d.Connect()
	if err != nil {
		t.Fatalf("Connecting: %v", err)
	}

	if fa, _ := dc.ForwardTCP(a); fa != a {
		t.Fatalf("expected '%s', got '%s'", a, fa)
	}
}

func TestForwardTCPBadAddress(t *testing.T) {
	t.Parallel()

	d := newDirect(t)

	dc, err := d.Connect()
	if err != nil {
		t.Fatalf("Connecting: %v", err)
	}

	a := "localhost"

	if _, err := dc.ForwardTCP(a); err == nil {
		t.Fatalf("TCP forwarding should fail when forwarding bad address")
	}
}
