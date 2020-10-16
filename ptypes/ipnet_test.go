package ptypes

import (
	"bytes"
	"encoding/json"
	"net"
	"testing"
)

func TestIPNetMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		input        IPNet
		expectedJSON string
	}{
		{
			input:        IPNet{net.IPNet{IP: net.IP{127, 0, 0, 1}, Mask: net.IPMask{255, 255, 255, 0}}},
			expectedJSON: `"127.0.0.1/24"`,
		},
		{
			input:        IPNet{net.IPNet{IP: net.IP{192, 168, 1, 2}, Mask: net.IPMask{255, 255, 255, 255}}},
			expectedJSON: `"192.168.1.2/32"`,
		},
	}

	for _, tt := range tests {
		output, err := json.Marshal(tt.input)
		if err != nil {
			t.Fatal("couldnt marshal input")
		}

		if string(output) != tt.expectedJSON {
			t.Fatalf("expectedJSON mismatch output, expected %s, got %s", tt.expectedJSON, string(output))
		}

		var revert IPNet

		err = json.Unmarshal(output, &revert)
		if err != nil {
			t.Fatal("couldnt unmarshal input")
		}

		if !bytes.Equal(revert.IPNet.IP, tt.input.IPNet.IP) {
			t.Fatal("original IPNet IP is different to marshal/unmarshalled input IP")
		}

		if !bytes.Equal(revert.IPNet.Mask, tt.input.IPNet.Mask) {
			t.Fatal("original IPNet Mask is different to marshal/unmarshalled input Mask")
		}
	}
}
