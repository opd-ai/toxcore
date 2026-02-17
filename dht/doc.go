// Package dht implements the Distributed Hash Table for the Tox protocol,
// providing peer discovery, routing, and group announcement functionality
// based on a modified Kademlia algorithm.
//
// # Architecture
//
// The DHT enables decentralized peer discovery without relying on central
// servers. Each node maintains a routing table organized into k-buckets,
// with nodes grouped by their XOR distance from the local node's public key.
//
// Key components:
//
//   - RoutingTable: Manages k-buckets for efficient node lookup
//   - BootstrapManager: Handles initial network connection and node discovery
//   - Maintainer: Performs periodic maintenance (pings, lookups, pruning)
//   - LANDiscovery: Discovers peers on the local network via UDP broadcast
//   - GroupStorage: Stores and queries group chat announcements
//
// # Bootstrap Process
//
// Initial connection to the Tox network requires bootstrapping from known nodes:
//
//	manager := dht.NewBootstrapManager(selfID, transport, routingTable)
//	err := manager.AddBootstrapNode("node.tox.biribiri.org", 33445,
//	    "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	err = manager.Bootstrap(ctx)
//
// The bootstrap manager includes exponential backoff for failed attempts and
// version negotiation for protocol compatibility.
//
// # Routing Table
//
// The routing table implements Kademlia-style k-buckets with configurable size
// (default: 8 nodes per bucket). Nodes are organized by XOR distance:
//
//	table := dht.NewRoutingTable(selfID, 8, 160)
//	table.AddNode(node)
//	closest := table.FindClosest(targetID, 8)
//
// The FindClosest method uses a min-heap for efficient retrieval of the k
// closest nodes to any target ID.
//
// # Node Status
//
// Nodes transition through three states based on responsiveness:
//
//	const (
//	    StatusUnknown NodeStatus = iota  // New node, untested
//	    StatusBad                        // Unresponsive, pending removal
//	    StatusGood                       // Actively responding to pings
//	)
//
// # DHT Maintenance
//
// The Maintainer performs periodic tasks to keep the routing table healthy:
//
//	config := dht.DefaultMaintenanceConfig()
//	maintainer := dht.NewMaintainer(routingTable, bootstrap, transport, selfNode, config)
//	maintainer.Start()
//	defer maintainer.Stop()
//
// Maintenance tasks include:
//   - Pinging nodes to verify liveness (default: every 1 minute)
//   - Random lookups to discover new nodes (default: every 5 minutes)
//   - Removing unresponsive nodes (bad timeout: 10 minutes)
//   - Pruning stale entries (prune timeout: 1 hour)
//
// # LAN Discovery
//
// Local network peer discovery uses UDP broadcast for quick connection to
// nearby Tox clients without requiring bootstrap servers:
//
//	discovery := dht.NewLANDiscovery(publicKey, 33445)
//	discovery.OnPeer(func(pk [32]byte, addr net.Addr) {
//	    // Handle discovered peer
//	})
//	discovery.Start()
//	defer discovery.Stop()
//
// LAN discovery broadcasts on port+1 to avoid conflicts with the main
// transport, with a 10-second interval between announcements.
//
// # Group Announcements
//
// The DHT supports storing and querying group chat announcements:
//
//	storage := dht.NewGroupStorage()
//	storage.StoreAnnouncement(&dht.GroupAnnouncement{
//	    GroupID:   12345,
//	    Name:      "My Group",
//	    Type:      1,
//	    Timestamp: time.Now(),
//	    TTL:       24 * time.Hour,
//	})
//
//	announcement := storage.GetAnnouncement(12345)
//
// # Multi-Network Support
//
// The DHT supports alternative network types through address detection:
//
//   - IP networks (IPv4, IPv6)
//   - Tor (.onion addresses)
//   - I2P (.i2p addresses)
//   - Nym (.nym addresses)
//   - Lokinet (.loki addresses)
//
// Address type detection is performed via AddressDetector which identifies
// network types without relying on DNS resolution.
//
// # Transport Integration
//
// The DHT integrates with the transport layer via the transport.Transport
// interface, supporting both UDP and TCP:
//
//	udpTransport, _ := transport.NewUDPTransport(":33445")
//	manager := dht.NewBootstrapManager(selfID, udpTransport, routingTable)
//
// Packet handlers are registered for DHT-specific packet types including
// ping requests/responses, node lookups, and group queries.
//
// # Thread Safety
//
// All DHT components use sync.RWMutex for concurrent access safety:
//   - RoutingTable: Protected bucket operations
//   - BootstrapManager: Thread-safe node management
//   - Maintainer: Safe start/stop lifecycle
//   - LANDiscovery: Concurrent-safe peer callbacks
//   - GroupStorage: Protected announcement storage
//
// # Deterministic Testing
//
// For reproducible test scenarios, use the TimeProvider interface:
//
//	dht.SetDefaultTimeProvider(&MockTimeProvider{currentTime: fixedTime})
//
// Individual components also support time injection:
//
//	manager.SetTimeProvider(mockTimeProvider)
//	maintainer.SetTimeProvider(mockTimeProvider)
//	node := dht.NewNodeWithTimeProvider(id, addr, mockTimeProvider)
//
// The TimeProvider allows injection of controlled time values for testing
// node freshness, maintenance timing, and bootstrap timestamp handling.
//
// # Version Negotiation
//
// Protocol version negotiation ensures backward compatibility. The bootstrap
// process includes version discovery to determine peer capabilities before
// establishing full connections.
package dht
