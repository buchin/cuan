package runner

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
)

type Runner struct {
	cfg     *config.Config
	adapter agent.Adapter
}

func NewRunner(cfg *config.Config, adapter agent.Adapter) *Runner {
	return &Runner{
		cfg:     cfg,
		adapter: adapter,
	}
}

// Start runs the worker polling loop until ctx is canceled.
func (r *Runner) Start(ctx context.Context) error {
	version, err := r.adapter.Version()
	if err != nil {
		return fmt.Errorf("check agent adapter: %w", err)
	}

	log.Printf("[Runner %s] Started with agent adapter: %s (version: %s)", r.cfg.RunnerID, r.adapter.Name(), version)
	log.Printf("[Runner %s] Polling queue every %v", r.cfg.RunnerID, r.cfg.RunnerInterval)

	ticker := time.NewTicker(r.cfg.RunnerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Runner %s] Context canceled, stopping worker loop...", r.cfg.RunnerID)
			return nil
		case <-ticker.C:
			// Heartbeat & poll loop placeholder
			log.Printf("[Runner %s] Heartbeat ok; task queue idle", r.cfg.RunnerID)
		}
	}
}
