package scriptmgr

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"send2nlm/sdk"

	"github.com/fsnotify/fsnotify"
)

// LoadProducerDir scans a directory for .go producer scripts,
// evaluates them, and registers all found producers.
func LoadProducerDir(engine *Engine, registry *ProducerRegistry, dir string) {
	producers := evaluateProducerDir(engine, dir)
	registry.SetScripts(producers)
	log.Printf("[scriptmgr] loaded %d producer script(s) from %s", len(producers), dir)
}

// LoadReceiverDir scans a directory for .go receiver scripts,
// evaluates them, and registers all found receivers.
func LoadReceiverDir(engine *Engine, registry *ReceiverRegistry, dir string) {
	receivers := evaluateReceiverDir(engine, dir)
	registry.SetScripts(receivers)
	log.Printf("[scriptmgr] loaded %d receiver script(s) from %s", len(receivers), dir)
}

// WatchProducerDir starts a goroutine that watches the producer script directory for changes.
func WatchProducerDir(engine *Engine, registry *ProducerRegistry, dir string) error {
	return watchDir(engine, dir, func() {
		producers := evaluateProducerDir(engine, dir)
		registry.SetScripts(producers)
		log.Printf("[scriptmgr] reloaded %d producer script(s) from %s", len(producers), dir)
	})
}

// WatchReceiverDir starts a goroutine that watches the receiver script directory for changes.
func WatchReceiverDir(engine *Engine, registry *ReceiverRegistry, dir string) error {
	return watchDir(engine, dir, func() {
		receivers := evaluateReceiverDir(engine, dir)
		registry.SetScripts(receivers)
		log.Printf("[scriptmgr] reloaded %d receiver script(s) from %s", len(receivers), dir)
	})
}

func watchDir(engine *Engine, dir string, reload func()) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return err
	}

	go func() {
		defer watcher.Close()
		// Debounce rapid events
		var debounce *fsnotify.Event
		for {
			select {
			case evt, ok := <-watcher.Events:
				if !ok {
					return
				}
				if strings.HasSuffix(evt.Name, ".go") {
					debounce = &evt
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[scriptmgr] watcher error: %v", err)
			}
			// Apply after all pending events are drained
			if debounce != nil {
				reload()
				debounce = nil
			}
		}
	}()

	return nil
}

func evaluateProducerDir(engine *Engine, dir string) []sdk.Producer {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("[scriptmgr] cannot read producer dir %s: %v", dir, err)
		return nil
	}

	var producers []sdk.Producer
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		p, err := loadProducer(engine, path)
		if err != nil {
			log.Printf("[scriptmgr] skip producer %s: %v", entry.Name(), err)
			continue
		}
		log.Printf("[scriptmgr] loaded producer %q from %s", p.Name(), entry.Name())
		producers = append(producers, p)
	}
	return producers
}

func evaluateReceiverDir(engine *Engine, dir string) []sdk.Receiver {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("[scriptmgr] cannot read receiver dir %s: %v", dir, err)
		return nil
	}

	var receivers []sdk.Receiver
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		r, err := loadReceiver(engine, path)
		if err != nil {
			log.Printf("[scriptmgr] skip receiver %s: %v", entry.Name(), err)
			continue
		}
		log.Printf("[scriptmgr] loaded receiver %q from %s", r.Name(), entry.Name())
		receivers = append(receivers, r)
	}
	return receivers
}

func loadProducer(engine *Engine, path string) (sdk.Producer, error) {
	i, err := engine.EvalScript(path)
	if err != nil {
		return nil, err
	}
	return ExtractProducer(i)
}

func loadReceiver(engine *Engine, path string) (sdk.Receiver, error) {
	i, err := engine.EvalScript(path)
	if err != nil {
		return nil, err
	}
	return ExtractReceiver(i)
}
