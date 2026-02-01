package retention

import (
	"log"
	"time"
)

func Start(enabled bool, interval time.Duration, logger *log.Logger) func() {
	if !enabled {
		return func() {}
	}

	stopCh := make(chan struct{})
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if logger != nil {
					logger.Printf("retention tick (placeholder)")
				}
			case <-stopCh:
				return
			}
		}
	}()

	return func() { close(stopCh) }
}
