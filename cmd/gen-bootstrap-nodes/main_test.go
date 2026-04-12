package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testJSON = `{
  "last_scan": 1700000000,
  "last_refresh": 1700000000,
  "nodes": [
    {
      "ipv4": "1.2.3.4",
      "ipv6": "::1",
      "port": 33445,
      "public_key": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
      "maintainer": "Alice",
      "status_udp": true,
      "last_ping": 1700000010
    },
    {
      "ipv4": "5.6.7.8",
      "ipv6": "-",
      "port": 33445,
      "public_key": "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
      "maintainer": "Bob",
      "status_udp": true,
      "last_ping": 1700000020
    },
    {
      "ipv4": "9.10.11.12",
      "ipv6": "-",
      "port": 33445,
      "public_key": "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
      "maintainer": "Charlie",
      "status_udp": false,
      "last_ping": 1700000030
    },
    {
      "ipv4": "13.14.15.16",
      "ipv6": "-",
      "port": 33445,
      "public_key": "DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD",
      "maintainer": "Dave (bad key)",
      "status_udp": true,
      "last_ping": 1700000040
    },
    {
      "ipv4": "",
      "ipv6": "::1",
      "port": 33445,
      "public_key": "EEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE",
      "maintainer": "Eve (no IPv4)",
      "status_udp": true,
      "last_ping": 1700000050
    },
    {
      "ipv4": "17.18.19.20",
      "ipv6": "-",
      "port": 0,
      "public_key": "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
      "maintainer": "Frank (port 0)",
      "status_udp": true,
      "last_ping": 1700000060
    },
    {
      "ipv4": "21.22.23.24",
      "ipv6": "-",
      "port": 33446,
      "public_key": "1111111111111111111111111111111111111111111111111111111111111111",
      "maintainer": "Grace",
      "status_udp": true,
      "last_ping": 1700000005
    },
    {
      "ipv4": "25.26.27.28",
      "ipv6": "-",
      "port": 33447,
      "public_key": "2222222222222222222222222222222222222222222222222222222222222222",
      "maintainer": "Heidi",
      "status_udp": true,
      "last_ping": 1700000015
    }
  ]
}`

func TestParseAndFilter(t *testing.T) {
	nodes, err := parseAndFilter([]byte(testJSON))
	require.NoError(t, err)

	// Should filter out:
	// - Charlie (status_udp=false)
	// - Dave (public key too long, 68 chars)
	// - Eve (empty IPv4)
	// - Frank (port 0)
	// Leaving: Bob (ping 20), Heidi (ping 15), Alice (ping 10), Grace (ping 5)
	require.Len(t, nodes, 4, "expected 4 valid nodes after filtering")

	// Verify sort order: descending by last_ping
	assert.Equal(t, "Bob", nodes[0].Maintainer)
	assert.Equal(t, "Heidi", nodes[1].Maintainer)
	assert.Equal(t, "Alice", nodes[2].Maintainer)
	assert.Equal(t, "Grace", nodes[3].Maintainer)

	// Verify fields
	assert.Equal(t, "5.6.7.8", nodes[0].IPV4)
	assert.Equal(t, uint16(33445), nodes[0].Port)
	assert.Equal(t, "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB", nodes[0].PublicKey)
}

func TestParseAndFilter_InvalidJSON(t *testing.T) {
	_, err := parseAndFilter([]byte(`{invalid`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JSON parse error")
}

func TestParseAndFilter_EmptyNodes(t *testing.T) {
	nodes, err := parseAndFilter([]byte(`{"nodes": []}`))
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestParseAndFilter_LowercaseKeysNormalized(t *testing.T) {
	data := []byte(`{"nodes": [
		{
			"ipv4": "1.2.3.4",
			"port": 33445,
			"public_key": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"maintainer": "Test",
			"status_udp": true,
			"last_ping": 100
		}
	]}`)
	nodes, err := parseAndFilter(data)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", nodes[0].PublicKey)
}

func TestGenerateSource(t *testing.T) {
	nodes := []toxNode{
		{IPV4: "1.2.3.4", Port: 33445, PublicKey: "AABB", Maintainer: "Test"},
	}
	src := generateSource(nodes)
	assert.Contains(t, src, "package nodes")
	assert.Contains(t, src, "DO NOT EDIT")
	assert.Contains(t, src, `Address: "1.2.3.4"`)
	assert.Contains(t, src, `Port: 33445`)
	assert.Contains(t, src, `PublicKey: "AABB"`)
	assert.Contains(t, src, `Maintainer: "Test"`)
	assert.Contains(t, src, "var DefaultNodes = []NodeInfo{")
	assert.NotContains(t, src, "Generated:")
}

func TestParseAndFilter_InvalidHexKey(t *testing.T) {
	// A 64-char key that contains non-hex characters (G, Z) should be filtered out
	data := []byte(`{"nodes": [
		{
			"ipv4": "1.2.3.4",
			"port": 33445,
			"public_key": "GGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG",
			"maintainer": "Bad hex",
			"status_udp": true,
			"last_ping": 100
		},
		{
			"ipv4": "5.6.7.8",
			"port": 33445,
			"public_key": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			"maintainer": "Good hex",
			"status_udp": true,
			"last_ping": 200
		}
	]}`)
	nodes, err := parseAndFilter(data)
	require.NoError(t, err)
	require.Len(t, nodes, 1, "node with invalid hex key should be filtered out")
	assert.Equal(t, "Good hex", nodes[0].Maintainer)
}
