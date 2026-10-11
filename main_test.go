package main

import "testing"

func TestListenAddress(t *testing.T) {
	t.Run("SV-14 an address that is set wins, whatever the socket flag says", func(t *testing.T) {
		for _, uds := range []bool{false, true} {
			if got := listenAddress("127.0.0.1:9000", uds); got != "127.0.0.1:9000" {
				t.Errorf("listenAddress(_, %t) = %q, want 127.0.0.1:9000", uds, got)
			}
		}
	})

	t.Run("SV-15 without an address a TCP server listens on every interface at port 8090", func(t *testing.T) {
		if got := listenAddress("", false); got != "0.0.0.0:8090" {
			t.Errorf("listenAddress = %q, want 0.0.0.0:8090", got)
		}
	})

	t.Run("SV-16 without an address a socket server uses /tmp/go.socket", func(t *testing.T) {
		if got := listenAddress("", true); got != "/tmp/go.socket" {
			t.Errorf("listenAddress = %q, want /tmp/go.socket", got)
		}
	})
}
