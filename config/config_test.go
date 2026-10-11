package config

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	t.Run("SV-17 an http or https URL is accepted and returned", func(t *testing.T) {
		for _, raw := range []string{"http://api.test", "https://api.test", "https://api.test:8443", "http://127.0.0.1:1", "http://[::1]:65535", "https://api.test/base"} {
			got, err := parseURL("NN_API_URL", raw)
			if err != nil || got != raw {
				t.Errorf("parseURL(%q) = %q, %v, want it back unchanged", raw, got, err)
			}
		}
	})

	t.Run("SV-18 surrounding whitespace and trailing slashes are removed", func(t *testing.T) {
		for raw, want := range map[string]string{
			"  http://api.test  ":      "http://api.test",
			"http://api.test/":         "http://api.test",
			"http://api.test///":       "http://api.test",
			"\thttps://api.test/v1/\n": "https://api.test/v1",
		} {
			got, err := parseURL("NN_API_URL", raw)
			if err != nil || got != want {
				t.Errorf("parseURL(%q) = %q, %v, want %q", raw, got, err, want)
			}
		}
	})

	t.Run("SV-19 an empty or blank value is refused with the variable name", func(t *testing.T) {
		for _, raw := range []string{"", "   ", "\t\n"} {
			_, err := parseURL("NN_SITE_URL", raw)
			if err == nil || err.Error() != "NN_SITE_URL environment variable must be set" {
				t.Errorf("parseURL(%q) error = %v, want the must-be-set message", raw, err)
			}
		}
	})

	t.Run("SV-20 a value that is not an http or https URL with a host is refused and quoted", func(t *testing.T) {
		for _, raw := range []string{"api.test", "ftp://api.test", "//api.test", "http://", "http:///path", "javascript:alert(1)", "http://api.test\x7f", "http://%zz"} {
			_, err := parseURL("NN_API_URL", raw)
			if err == nil || !strings.HasPrefix(err.Error(), "NN_API_URL must be an http(s) URL, got ") {
				t.Errorf("parseURL(%q) error = %v, want the not-an-URL message", raw, err)
			}
		}
		_, err := parseURL("NN_API_URL", "  ftp://api.test  ")
		if err == nil || err.Error() != `NN_API_URL must be an http(s) URL, got "ftp://api.test"` {
			t.Errorf("error = %v, want the trimmed value quoted", err)
		}
	})

	t.Run("SV-21 a port outside 1 to 65535 or not a number is refused", func(t *testing.T) {
		for _, raw := range []string{"http://api.test:0", "http://api.test:65536", "http://api.test:99999999999", "http://api.test:abc", "http://api.test:-1"} {
			if _, err := parseURL("NN_API_URL", raw); err == nil {
				t.Errorf("parseURL(%q) accepted the port", raw)
			}
		}
	})
}

func TestValidPort(t *testing.T) {
	t.Run("SV-22 no port and the ports 1 to 65535 are valid, others are not", func(t *testing.T) {
		cases := map[string]bool{
			"": true, "1": true, "80": true, "65535": true,
			"0": false, "65536": false, "-1": false, "abc": false, "8 0": false,
		}
		for port, want := range cases {
			if got := validPort(port); got != want {
				t.Errorf("validPort(%q) = %t, want %t", port, got, want)
			}
		}
	})
}

func TestRequireURLEndsTheProcess(t *testing.T) {
	run := func(t *testing.T, env ...string) (int, string) {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^$")
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode(), string(out)
		}
		if err != nil {
			t.Fatal(err)
		}
		return 0, string(out)
	}

	t.Run("SV-23 a missing variable ends the process with status 1 and says which", func(t *testing.T) {
		code, out := run(t, "NN_API_URL=", "NN_SITE_URL=http://site.test")
		if code != 1 || !strings.Contains(out, "NN_API_URL environment variable must be set") {
			t.Errorf("exit %d, output %q, want 1 and the must-be-set message", code, out)
		}
	})

	t.Run("SV-24 a variable that is not an URL ends the process with status 1 and says which", func(t *testing.T) {
		code, out := run(t, "NN_API_URL=http://api.test", "NN_SITE_URL=ftp://site.test")
		if code != 1 || !strings.Contains(out, `NN_SITE_URL must be an http(s) URL, got "ftp://site.test"`) {
			t.Errorf("exit %d, output %q, want 1 and the not-an-URL message", code, out)
		}
	})

	t.Run("SV-25 valid variables let the process start", func(t *testing.T) {
		code, out := run(t, "NN_API_URL=http://api.test/", "NN_SITE_URL=https://site.test")
		if code != 0 {
			t.Errorf("exit %d, output %q, want 0", code, out)
		}
	})
}
