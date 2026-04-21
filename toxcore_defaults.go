package toxcore

import (
	"fmt"

	"github.com/opd-ai/toxcore/bootstrap/nodes"
	"github.com/sirupsen/logrus"
)

// BootstrapDefaults connects to the default Tox DHT bootstrap nodes.
// It validates, resolves, and adds all default nodes to the bootstrap
// manager first, then executes the bootstrap process a single time.
// Returns an error only if no nodes could be added or the bootstrap fails.
func (t *Tox) BootstrapDefaults() error {
	if len(nodes.DefaultNodes) == 0 {
		return fmt.Errorf("no default bootstrap nodes available")
	}

	var lastAddErr error
	var addedCount int

	// Phase 1: validate, resolve, and add all default nodes to the manager.
	for _, node := range nodes.DefaultNodes {
		if err := t.tryAddDefaultNode(node); err != nil {
			lastAddErr = err
			continue
		}
		addedCount++
	}

	if addedCount == 0 {
		return fmt.Errorf("all %d default bootstrap nodes failed to add, last error: %w", len(nodes.DefaultNodes), lastAddErr)
	}

	// Phase 2: execute the bootstrap process once with all added nodes.
	if err := t.executeBootstrapProcess("defaults", 0); err != nil {
		return fmt.Errorf("bootstrap process failed after adding %d nodes: %w", addedCount, err)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "BootstrapDefaults",
		"added_count": addedCount,
		"total_nodes": len(nodes.DefaultNodes),
	}).Info("Default bootstrap completed")

	return nil
}

// tryAddDefaultNode validates, resolves, and adds a single default bootstrap node.
// Returns an error if any step fails, logging the failure at Debug level.
func (t *Tox) tryAddDefaultNode(node nodes.NodeInfo) error {
	if err := validateBootstrapPublicKey(node.PublicKey, node.Address, node.Port); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "BootstrapDefaults",
			"address":  node.Address,
			"port":     node.Port,
			"error":    err.Error(),
		}).Debug("Default bootstrap node key validation failed")
		return err
	}

	addr, err := resolveBootstrapAddress(node.Address, node.Port)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "BootstrapDefaults",
			"address":  node.Address,
			"port":     node.Port,
			"error":    err.Error(),
		}).Debug("Default bootstrap node address resolution failed")
		return err
	}

	if err := t.addBootstrapNode(addr, node.PublicKey); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "BootstrapDefaults",
			"address":  node.Address,
			"port":     node.Port,
			"error":    err.Error(),
		}).Debug("Default bootstrap node add failed")
		return err
	}

	return nil
}
