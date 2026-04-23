package cli

import "testing"

func TestParsePositionalArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		args     []string
		listen   string
		root     string
		upstream string
	}{
		{name: "default", args: nil, root: ".", listen: ":8000"},
		{name: "path only", args: []string{"."}, root: ".", listen: ":8000"},
		{name: "listen only", args: []string{":8080"}, root: ".", listen: ":8080"},
		{name: "bare listen", args: []string{"8080"}, root: ".", listen: ":8080"},
		{name: "alias pair", args: []string{"80:8080"}, root: ".", listen: ":80", upstream: "127.0.0.1:8080"},
		{name: "canonical pair", args: []string{"0.0.0.0:80=127.0.0.1:8080"}, root: ".", listen: "0.0.0.0:80", upstream: "127.0.0.1:8080"},
		{name: "path and listen", args: []string{"/tmp", ":8080"}, root: "/tmp", listen: ":8080"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParsePositionalArgs(tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Root != tc.root {
				t.Fatalf("root: got %q want %q", got.Root, tc.root)
			}
			if got.Listen != tc.listen {
				t.Fatalf("listen: got %q want %q", got.Listen, tc.listen)
			}
			if got.Upstream != tc.upstream {
				t.Fatalf("upstream: got %q want %q", got.Upstream, tc.upstream)
			}
		})
	}
}

func TestParsePositionalArgsErrors(t *testing.T) {
	t.Parallel()
	cases := [][]string{
		{"a", "b", "c"},
		{"80:8080", "81:8181"},
		{"a", "b"},
	}

	for _, args := range cases {
		args := args
		t.Run(args[0], func(t *testing.T) {
			t.Parallel()
			if _, err := ParsePositionalArgs(args); err == nil {
				t.Fatalf("expected error for args=%v", args)
			}
		})
	}
}
