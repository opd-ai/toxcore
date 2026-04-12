package toxcore

import (
	"fmt"

	"github.com/opd-ai/toxcore/bootstrap/nodes"
	"github.com/sirupsen/logrus"
)

// BootstrapDefaults connects to the default Tox DHT bootstrap nodes.
// It iterates over nodes.DefaultNodes and calls Bootstrap for each.
// Returns an error only if all nodes fail to bootstrap.
func (t *Tox) BootstrapDefaults() error {
	if len(nodes.DefaultNodes) == 0 {
		return fmt.Errorf("no default bootstrap nodes available")
	}

	var lastErr error
	var successCount int

	for _, node := range nodes.DefaultNodes {
		err := t.Bootstrap(node.Address, node.Port, node.PublicKey)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "BootstrapDefaults",
				"address":  node.Address,
				"port":     node.Port,
				"error":    err.Error(),
			}).Debug("Default bootstrap node failed")
			lastErr = err
			continue
		}
		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("all %d default bootstrap nodes failed, last error: %w", len(nodes.DefaultNodes), lastErr)
	}

	logrus.WithFields(logrus.Fields{
		"function":      "BootstrapDefaults",
		"success_count": successCount,
		"total_nodes":   len(nodes.DefaultNodes),
	}).Info("Default bootstrap completed")

	return nil
}
