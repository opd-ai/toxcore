# DHT Routing Table Architecture

This document describes the Kademlia-based DHT (Distributed Hash Table) routing table implementation in toxcore-go.

## Overview

toxcore-go implements a standard Kademlia DHT for peer discovery and routing. The routing table uses k-buckets to organize known peers based on their XOR distance from the local node.

## Key Constants

| Constant | Value | Description |
|----------|-------|-------------|
| Number of Buckets | 256 | One bucket per bit of the 256-bit public key space |
| Default Bucket Size | 8 | Base k-bucket size (MinBucketSize) |
| Maximum Bucket Size | 64 | Upper limit per bucket to prevent unbounded growth |
| Maximum Routing Table Capacity | 16,384 | 256 buckets × 64 nodes/bucket |
| Lookup Cache TTL | 30 seconds | Time-to-live for cached lookup results |
| Lookup Cache Max Size | 256 entries | Maximum cached FindClosestNodes results |

## Routing Table Structure

```
┌──────────────────────────────────────────────────────────┐
│                   RoutingTable                           │
├──────────────────────────────────────────────────────────┤
│  selfID        ToxID      (our 32-byte public key)       │
│  kBuckets[256] *KBucket   (one bucket per distance bit)  │
│  maxNodes      int        (total capacity)               │
│  lookupCache   *LookupCache  (LRU cache for queries)     │
│  nodeCount     int32      (current node count, atomic)   │
└──────────────────────────────────────────────────────────┘
                            │
                            ▼
┌──────────────────────────────────────────────────────────┐
│                      KBucket                             │
├──────────────────────────────────────────────────────────┤
│  nodes    []*Node    (nodes in this bucket)              │
│  maxSize  int        (bucket capacity, 8-64)             │
│  mu       sync.RWMutex  (thread-safe access)             │
└──────────────────────────────────────────────────────────┘
```

## Bucket Index Calculation

The bucket index is determined by the XOR distance between the local node ID and the target node ID:

```go
func computeBucketIndex(selfID crypto.ToxID, node *Node) int {
    // XOR the two public keys
    xorResult := xorBytes(selfID.PublicKey[:], node.PublicKey[:])
    
    // Find the leading zero bits count
    // Bucket 0 = closest nodes (most leading zeros)
    // Bucket 255 = farthest nodes (no leading zeros)
    return 255 - leadingZeroBits(xorResult)
}
```

## Dynamic Bucket Sizing

toxcore-go implements dynamic bucket sizing based on observed network density:

- **DensityEstimator** tracks node addition patterns over a 5-minute window
- Buckets expand (up to 64 nodes) when network density is high (>90% fill rate)
- Buckets contract (down to 8 nodes) when density is low (<30% fill rate)
- Minimum 20 nodes required before density estimation kicks in

This allows the routing table to adapt to different network sizes while preventing excessive memory usage on sparse networks.

## Scalability Characteristics

### Recommended Network Sizes

| Network Size | Bucket Fill | Memory Estimate | Lookup Latency |
|--------------|-------------|-----------------|----------------|
| < 1,000 nodes | Sparse | ~2 KB | O(log n) |
| 1,000 - 10,000 | Moderate | ~20 KB | O(log n) |
| 10,000 - 100,000 | Dense | ~200 KB | O(log n) |
| > 100,000 | Very Dense | ~1 MB | O(log n) |

### Lookup Performance

- **FindClosestNodes**: Returns up to `count` nodes closest to a target key
- **Caching**: LRU cache (256 entries, 30s TTL) reduces repeated lookups
- **Parallelism**: Multiple lookups can run concurrently (read-lock on buckets)

### Network Capacity

The theoretical maximum is 16,384 nodes (256 × 64), but in practice:

- Global Tox network: Typically 5,000-15,000 active nodes
- Private deployments: Usually 10-1,000 nodes
- The routing table handles both extremes efficiently via dynamic sizing

## Thread Safety

The routing table is fully thread-safe:

- `sync.RWMutex` protects each k-bucket individually
- `sync/atomic` operations for node count
- Lock-free reads of stable bucket references

## Lookup Cache

The `LookupCache` implements an LRU (Least Recently Used) cache:

- Reduces redundant DHT queries for frequently accessed targets
- 30-second TTL prevents stale results
- 256-entry maximum prevents unbounded memory growth
- Statistics tracking: hits, misses, evictions

## Code Locations

| Component | File | Description |
|-----------|------|-------------|
| RoutingTable | `dht/routing.go` | Main routing table implementation |
| KBucket | `dht/routing.go` | Individual k-bucket implementation |
| LookupCache | `dht/routing.go` | LRU lookup cache |
| DensityEstimator | `dht/dynamic_bucket.go` | Network density tracking |
| Constants | `dht/dynamic_bucket.go` | Bucket size limits |

## Configuration

The routing table is configured at creation time:

```go
// Create routing table with custom bucket size
rt := dht.NewRoutingTable(selfID, maxBucketSize)

// maxBucketSize: nodes per bucket (8-64, default 8)
// Total capacity = maxBucketSize * 256
```

For most deployments, the default configuration is appropriate. Increase `maxBucketSize` only if:

1. Operating in a very dense network (>50,000 nodes)
2. Willing to trade memory for faster lookups
3. Have high-bandwidth connections to handle more peer traffic

## Best Practices

1. **Bootstrap First**: Connect to known nodes before relying on DHT lookups
2. **Handle Churn**: Nodes come and go; don't cache DHT results indefinitely
3. **Respect Rate Limits**: DHT operations have implicit rate limiting via the routing protocol
4. **Monitor Bucket Fill**: High fill rates indicate good network connectivity
