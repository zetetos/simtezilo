package updater //nolint:testpackage // testing internal functions

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    *Version
		wantErr bool
	}{
		{
			name:  "simple version",
			input: "1.2.3",
			want:  &Version{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "version with v prefix",
			input: "v1.2.3",
			want:  &Version{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "version with prerelease",
			input: "1.2.3-beta.1",
			want:  &Version{Major: 1, Minor: 2, Patch: 3, PreRelease: "beta.1"},
		},
		{
			name:  "version with v prefix and prerelease",
			input: "v2.0.0-alpha",
			want:  &Version{Major: 2, Minor: 0, Patch: 0, PreRelease: "alpha"},
		},
		{
			name:    "invalid version",
			input:   "not-a-version",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "incomplete version",
			input:   "1.2",
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseVersion(testCase.input)
			if (err != nil) != testCase.wantErr {
				t.Errorf("ParseVersion() error = %v, wantErr %v", err, testCase.wantErr)

				return
			}

			if testCase.wantErr {
				return
			}

			if got.Major != testCase.want.Major || got.Minor != testCase.want.Minor || got.Patch != testCase.want.Patch {
				t.Errorf("ParseVersion() = %v, want %v", got, testCase.want)
			}

			if got.PreRelease != testCase.want.PreRelease {
				t.Errorf("ParseVersion() PreRelease = %v, want %v", got.PreRelease, testCase.want.PreRelease)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		{name: "equal versions", v1: "1.2.3", v2: "1.2.3", want: 0},
		{name: "major less than", v1: "1.0.0", v2: "2.0.0", want: -1},
		{name: "major greater than", v1: "2.0.0", v2: "1.0.0", want: 1},
		{name: "minor less than", v1: "1.1.0", v2: "1.2.0", want: -1},
		{name: "patch less than", v1: "1.2.3", v2: "1.2.4", want: -1},
		{name: "prerelease less than release", v1: "1.0.0-alpha", v2: "1.0.0", want: -1},
		{name: "release greater than prerelease", v1: "1.0.0", v2: "1.0.0-beta", want: 1},
		{name: "prerelease alpha less than beta", v1: "1.0.0-alpha", v2: "1.0.0-beta", want: -1},
		{name: "numeric prerelease comparison", v1: "1.0.0-beta.1", v2: "1.0.0-beta.2", want: -1},
		{name: "v prefix handled", v1: "v1.0.0", v2: "1.0.0", want: 0},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			version1, err := ParseVersion(testCase.v1)
			if err != nil {
				t.Fatalf("Failed to parse v1: %v", err)
			}

			version2, err := ParseVersion(testCase.v2)
			if err != nil {
				t.Fatalf("Failed to parse v2: %v", err)
			}

			got := version1.Compare(version2)
			if got != testCase.want {
				t.Errorf("Compare() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple version", input: "1.2.3", want: "v1.2.3"},
		{name: "with prerelease", input: "1.0.0-beta.1", want: "v1.0.0-beta.1"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			v, _ := ParseVersion(testCase.input)
			if got := v.String(); got != testCase.want {
				t.Errorf("String() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestVersionLessThanGreaterThan(t *testing.T) {
	t.Parallel()

	version1, _ := ParseVersion("1.0.0")
	version2, _ := ParseVersion("2.0.0")

	if !version1.LessThan(version2) {
		t.Error("Expected 1.0.0 < 2.0.0")
	}

	if version2.LessThan(version1) {
		t.Error("Expected 2.0.0 not < 1.0.0")
	}

	if !version2.GreaterThan(version1) {
		t.Error("Expected 2.0.0 > 1.0.0")
	}

	if version1.GreaterThan(version2) {
		t.Error("Expected 1.0.0 not > 2.0.0")
	}

	// Test equality with a copy of the same version
	version1Copy, _ := ParseVersion("1.0.0")
	if !version1.Equal(version1Copy) {
		t.Error("Expected 1.0.0 == 1.0.0")
	}
}

func TestIsPreRelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "stable version", input: "1.0.0", want: false},
		{name: "beta version", input: "1.0.0-beta.1", want: true},
		{name: "dev version", input: "1.0.0-dev.42", want: true},
		{name: "alpha version", input: "1.0.0-alpha", want: true},
		{name: "rc version", input: "2.0.0-rc.1", want: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			v, err := ParseVersion(testCase.input)
			if err != nil {
				t.Fatalf("ParseVersion() error = %v", err)
			}

			if got := v.IsPreRelease(); got != testCase.want {
				t.Errorf("IsPreRelease() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestInferredChannel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "stable version", input: "1.0.0", want: "stable"},
		{name: "beta version with dot", input: "1.0.0-beta.1", want: "beta"},
		{name: "beta version without dot", input: "1.0.0-beta1", want: "beta"},
		{name: "dev version with dot", input: "1.0.0-dev.42", want: "dev"},
		{name: "dev version without dot", input: "1.0.0-dev42", want: "dev"},
		{name: "alpha version", input: "1.0.0-alpha", want: "alpha"},
		{name: "rc version", input: "2.0.0-rc.1", want: "rc"},
		{name: "rc version without dot", input: "2.0.0-rc1", want: "rc"},
		{name: "beta with uppercase", input: "1.0.0-BETA.1", want: "beta"},
		{name: "beta with uppercase no dot", input: "1.0.0-BETA1", want: "beta"},
		{name: "custom version", input: "1.0.0-custom", want: "custom"},
		{name: "custom with suffix", input: "1.0.0-custom-beta.1", want: "custom"},
		{name: "custom with beta suffix", input: "1.0.0-custom-mybuild", want: "custom"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			v, err := ParseVersion(testCase.input)
			if err != nil {
				t.Fatalf("ParseVersion() error = %v", err)
			}

			if got := v.InferredChannel(); got != testCase.want {
				t.Errorf("InferredChannel() = %v, want %v", got, testCase.want)
			}
		})
	}
}
