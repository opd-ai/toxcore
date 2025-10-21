package crypto

import (
	"math"
	"testing"
)

func TestSafeUint64ToInt64(t *testing.T) {
	tests := []struct {
		name    string
		input   uint64
		want    int64
		wantErr bool
	}{
		{
			name:    "zero value",
			input:   0,
			want:    0,
			wantErr: false,
		},
		{
			name:    "positive value",
			input:   12345,
			want:    12345,
			wantErr: false,
		},
		{
			name:    "max safe value",
			input:   math.MaxInt64,
			want:    math.MaxInt64,
			wantErr: false,
		},
		{
			name:    "overflow value",
			input:   math.MaxInt64 + 1,
			want:    0,
			wantErr: true,
		},
		{
			name:    "max uint64 value",
			input:   math.MaxUint64,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safeUint64ToInt64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("safeUint64ToInt64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("safeUint64ToInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafeInt64ToUint64(t *testing.T) {
	tests := []struct {
		name    string
		input   int64
		want    uint64
		wantErr bool
	}{
		{
			name:    "zero value",
			input:   0,
			want:    0,
			wantErr: false,
		},
		{
			name:    "positive value",
			input:   12345,
			want:    12345,
			wantErr: false,
		},
		{
			name:    "max int64 value",
			input:   math.MaxInt64,
			want:    math.MaxInt64,
			wantErr: false,
		},
		{
			name:    "negative value",
			input:   -1,
			want:    0,
			wantErr: true,
		},
		{
			name:    "large negative value",
			input:   math.MinInt64,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safeInt64ToUint64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("safeInt64ToUint64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("safeInt64ToUint64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafeDurationToUint64(t *testing.T) {
	tests := []struct {
		name    string
		input   int64
		want    uint64
		wantErr bool
	}{
		{
			name:    "zero duration",
			input:   0,
			want:    0,
			wantErr: false,
		},
		{
			name:    "positive duration",
			input:   1000000000, // 1 second in nanoseconds
			want:    1000000000,
			wantErr: false,
		},
		{
			name:    "max safe duration",
			input:   math.MaxInt64,
			want:    math.MaxInt64,
			wantErr: false,
		},
		{
			name:    "negative duration",
			input:   -1000000000,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safeDurationToUint64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("safeDurationToUint64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("safeDurationToUint64() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTimestampOverflowProtection verifies overflow protection as specified in audit
func TestTimestampOverflowProtection(t *testing.T) {
	// Test conversion of max values
	maxUint := uint64(math.MaxUint64)
	_, err := safeUint64ToInt64(maxUint)
	if err == nil {
		t.Error("should reject overflow")
	}

	// Test negative timestamp
	negInt := int64(-1)
	_, err = safeInt64ToUint64(negInt)
	if err == nil {
		t.Error("should reject negative")
	}

	// Test valid conversions
	validUint := uint64(math.MaxInt64)
	result, err := safeUint64ToInt64(validUint)
	if err != nil {
		t.Errorf("should accept max int64: %v", err)
	}
	if result != math.MaxInt64 {
		t.Errorf("incorrect conversion result: got %v, want %v", result, math.MaxInt64)
	}
}
