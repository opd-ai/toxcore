package internal

import "testing"

func TestPortConstants(t *testing.T) {
	// Verify port constants have expected values
	tests := []struct {
		name     string
		constant uint16
		expected uint16
	}{
		{"BootstrapDefaultPort", BootstrapDefaultPort, 33445},
		{"AlicePortRangeStart", AlicePortRangeStart, 33500},
		{"AlicePortRangeEnd", AlicePortRangeEnd, 33599},
		{"BobPortRangeStart", BobPortRangeStart, 33600},
		{"BobPortRangeEnd", BobPortRangeEnd, 33699},
		{"OtherPortRangeStart", OtherPortRangeStart, 33700},
		{"OtherPortRangeEnd", OtherPortRangeEnd, 33799},
		{"MinValidPort", MinValidPort, 1024},
		{"MaxValidPort", MaxValidPort, 65535},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.constant != tc.expected {
				t.Errorf("%s = %d; want %d", tc.name, tc.constant, tc.expected)
			}
		})
	}
}

func TestPortRangesDoNotOverlap(t *testing.T) {
	// Ensure the default port ranges don't overlap
	ranges := []struct {
		name  string
		start uint16
		end   uint16
	}{
		{"Bootstrap", BootstrapDefaultPort, BootstrapDefaultPort},
		{"Alice", AlicePortRangeStart, AlicePortRangeEnd},
		{"Bob", BobPortRangeStart, BobPortRangeEnd},
		{"Other", OtherPortRangeStart, OtherPortRangeEnd},
	}

	for i := 0; i < len(ranges); i++ {
		for j := i + 1; j < len(ranges); j++ {
			r1, r2 := ranges[i], ranges[j]
			// Check if ranges overlap
			if r1.start <= r2.end && r2.start <= r1.end {
				t.Errorf("Port ranges %s [%d-%d] and %s [%d-%d] overlap",
					r1.name, r1.start, r1.end,
					r2.name, r2.start, r2.end)
			}
		}
	}
}

func TestValidatePortRange(t *testing.T) {
	tests := []struct {
		name      string
		startPort uint16
		endPort   uint16
		want      bool
	}{
		{
			name:      "valid range",
			startPort: AlicePortRangeStart,
			endPort:   AlicePortRangeEnd,
			want:      true,
		},
		{
			name:      "single port",
			startPort: BootstrapDefaultPort,
			endPort:   BootstrapDefaultPort,
			want:      true,
		},
		{
			name:      "start greater than end",
			startPort: 5000,
			endPort:   4000,
			want:      false,
		},
		{
			name:      "privileged port start",
			startPort: 80,
			endPort:   8080,
			want:      false,
		},
		{
			name:      "port zero",
			startPort: 0,
			endPort:   1000,
			want:      false,
		},
		{
			name:      "max port boundary",
			startPort: 65000,
			endPort:   65535,
			want:      true,
		},
		{
			name:      "minimum valid port",
			startPort: MinValidPort,
			endPort:   MinValidPort + 100,
			want:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidatePortRange(tc.startPort, tc.endPort)
			if got != tc.want {
				t.Errorf("ValidatePortRange(%d, %d) = %v; want %v",
					tc.startPort, tc.endPort, got, tc.want)
			}
		})
	}
}
