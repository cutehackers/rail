package runtime

import (
	"io"
	"sync"
	"time"
)

type ActorWatchdogConfig struct {
	QuietWindow time.Duration
	CheckEvery  time.Duration
}

type ActorWatchdogExpiration struct {
	ActorName   string
	QuietWindow time.Duration
}

type ActorWatchdog struct {
	actorName string
	config    ActorWatchdogConfig
	stopped   chan struct{}
	done      chan struct{}
	stopOnce  sync.Once

	mu           sync.Mutex
	lastProgress time.Time
	expiration   *ActorWatchdogExpiration
}

func productionActorWatchdogConfig() ActorWatchdogConfig {
	return ActorWatchdogConfig{
		QuietWindow: 10 * time.Minute,
		CheckEvery:  5 * time.Second,
	}
}

var defaultActorWatchdogConfig = productionActorWatchdogConfig()

func newActorWatchdog(actorName string, config ActorWatchdogConfig) *ActorWatchdog {
	if config.CheckEvery <= 0 {
		config.CheckEvery = 5 * time.Second
	}
	return &ActorWatchdog{
		actorName:    actorName,
		config:       config,
		stopped:      make(chan struct{}),
		done:         make(chan struct{}),
		lastProgress: time.Now(),
	}
}

func (w *ActorWatchdog) Start(cancel func()) {
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(w.config.CheckEvery)
		defer ticker.Stop()

		for {
			select {
			case <-w.stopped:
				return
			case <-ticker.C:
				if w.markExpiredIfQuiet() {
					cancel()
					return
				}
			}
		}
	}()
}

func (w *ActorWatchdog) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopped)
	})
	<-w.done
}

func (w *ActorWatchdog) ObserveProgress() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastProgress = time.Now()
}

func (w *ActorWatchdog) ProgressWriter() io.Writer {
	return actorWatchdogProgressWriter{watchdog: w}
}

func (w *ActorWatchdog) Expiration() (ActorWatchdogExpiration, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.expiration == nil {
		return ActorWatchdogExpiration{}, false
	}
	return *w.expiration, true
}

func (w *ActorWatchdog) markExpiredIfQuiet() bool {
	if w.config.QuietWindow <= 0 {
		return false
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.expiration != nil {
		return true
	}
	if time.Since(w.lastProgress) < w.config.QuietWindow {
		return false
	}
	w.expiration = &ActorWatchdogExpiration{
		ActorName:   w.actorName,
		QuietWindow: w.config.QuietWindow,
	}
	return true
}

type actorWatchdogProgressWriter struct {
	watchdog *ActorWatchdog
}

func (w actorWatchdogProgressWriter) Write(p []byte) (int, error) {
	if len(p) > 0 {
		w.watchdog.ObserveProgress()
	}
	return len(p), nil
}
