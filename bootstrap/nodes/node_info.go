package nodes

//go:generate go run ../../cmd/gen-bootstrap-nodes

// NodeInfo describes a Tox DHT bootstrap node.
type NodeInfo struct {
	Address    string // IPv4 address or hostname
	Port       uint16
	PublicKey  string // 64-char hex-encoded 32-byte key
	Maintainer string // human-readable maintainer name
}
