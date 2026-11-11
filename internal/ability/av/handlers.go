package av

import (
	"context"
	"errors"
	"log"
	"time"
)

// handleScan validates the payload, registers a scan run and executes it
// synchronously; core's loopback window for AV_SCAN is 4h
// (ipc.CommandTimeout). Progress is posted through prog, the terminal
// receipt carries the scan summary as result.
func (e *Engine) handleScan(msgId string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	opts, perr := parseScanPayload(payload, e.opts.DefaultOnDetect)
	if perr != "" {
		return false, "E_PARAM", perr, nil
	}
	e.mu.Lock()
	if e.scan != nil && e.scan.running.Load() {
		e.mu.Unlock()
		return false, "E_SCAN_RUNNING", "a scan is already running", nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &scanRun{
		id:      newScanID(),
		ctx:     ctx,
		cancel:  cancel,
		started: time.Now(),
	}
	run.running.Store(true)
	e.scan = run
	e.mu.Unlock()

	result, ok, code, message := e.runScan(run, opts, prog)
	e.mu.Lock()
	if e.scan == run {
		e.scan = nil
	}
	e.mu.Unlock()
	return ok, code, message, result
}

// handleStopScan cancels the running scan; the scan's own /execute call
// then returns with cancelled=true in its summary.
func (e *Engine) handleStopScan() (bool, string, string, map[string]any) {
	e.mu.Lock()
	run := e.scan
	e.mu.Unlock()
	if run == nil || !run.running.Load() {
		return false, "E_NO_SCAN", "no scan is running", nil
	}
	run.cancel()
	log.Printf("[av] stop requested for scan %s", run.id)
	return true, "", "", map[string]any{"stopRequested": true}
}

// handleStatus reports scan state, db info and quarantine count.
func (e *Engine) handleStatus() (bool, string, string, map[string]any) {
	e.mu.Lock()
	run := e.scan
	e.mu.Unlock()
	var scanState map[string]any
	if run != nil && run.running.Load() {
		scanState = map[string]any{
			"scanId":      run.id,
			"started":     run.started.UTC().Format(time.RFC3339),
			"totalSeen":   run.totalSeen.Load(),
			"processed":   run.processed.Load(),
			"bytesHashed": run.bytesHashed.Load(),
			"threats":     run.threats.Load(),
		}
	}
	items, _ := e.quarantine.List()
	return true, "", "", map[string]any{
		"scanning":   scanState != nil,
		"scan":       scanState,
		"db":         dbInfo(e.db.Load()),
		"quarantine": map[string]any{"dir": e.quarantine.Dir(), "count": len(items)},
	}
}

// dbInfo summarizes a signature DB for receipts.
func dbInfo(db *SignatureDB) map[string]any {
	if db == nil {
		return map[string]any{"loaded": false}
	}
	return map[string]any{
		"loaded":   true,
		"version":  db.Version,
		"updated":  db.Updated,
		"source":   db.Source,
		"hashes":   len(db.Hashes),
		"patterns": len(db.Patterns),
	}
}

// handleDBInfo returns the loaded signature DB summary.
func (e *Engine) handleDBInfo() (bool, string, string, map[string]any) {
	return true, "", "", dbInfo(e.db.Load())
}

// handleDBUpdate replaces the signature DB from a local file. The ability
// process has no network access, so the DB file must already be on disk
// (e.g. delivered by FILE_PUSH in M1, or updated by an administrator).
// The in-memory DB is only replaced after the new file validates.
func (e *Engine) handleDBUpdate(payload map[string]any) (bool, string, string, map[string]any) {
	path := str(payload, "path")
	if path == "" {
		return false, "E_PARAM", "path is required", nil
	}
	db, err := LoadSignatureDB(path)
	if err != nil {
		log.Printf("[av] db update rejected: %v", err)
		return false, "E_DB_FORMAT", "invalid signature db: " + err.Error(), nil
	}
	e.db.Store(db)
	if err := db.Save(e.dbPath); err != nil {
		log.Printf("[av] db update persisted in memory but failed to save: %v", err)
	}
	log.Printf("[av] signature db updated from %s: version=%d hashes=%d patterns=%d", path, db.Version, len(db.Hashes), len(db.Patterns))
	return true, "", "", dbInfo(db)
}

// handleQuarantineList returns the quarantine store listing.
func (e *Engine) handleQuarantineList() (bool, string, string, map[string]any) {
	items, err := e.quarantine.List()
	if err != nil {
		return false, "E_FAILED", err.Error(), nil
	}
	if items == nil {
		items = []QuarantineItem{}
	}
	return true, "", "", map[string]any{"items": items, "count": len(items)}
}

// handleQuarantineRestore restores one item by id.
func (e *Engine) handleQuarantineRestore(payload map[string]any) (bool, string, string, map[string]any) {
	id := str(payload, "id")
	if id == "" {
		return false, "E_PARAM", "id is required", nil
	}
	it, err := e.quarantine.Restore(id)
	if err != nil {
		if errors.Is(err, ErrQuarantineNotFound) {
			return false, "E_QUAR_NOT_FOUND", "no such quarantine item: " + id, nil
		}
		if errors.Is(err, ErrTargetExists) {
			return false, "E_TARGET_EXISTS", err.Error(), nil
		}
		return false, "E_FAILED", err.Error(), nil
	}
	log.Printf("[av] quarantine restore ok: %s -> %s", it.ID, it.OriginalPath)
	return true, "", "", map[string]any{"id": it.ID, "restored": it.OriginalPath}
}

// handleQuarantineDelete removes one item permanently.
func (e *Engine) handleQuarantineDelete(payload map[string]any) (bool, string, string, map[string]any) {
	id := str(payload, "id")
	if id == "" {
		return false, "E_PARAM", "id is required", nil
	}
	it, err := e.quarantine.Delete(id)
	if err != nil {
		if errors.Is(err, ErrQuarantineNotFound) {
			return false, "E_QUAR_NOT_FOUND", "no such quarantine item: " + id, nil
		}
		return false, "E_FAILED", err.Error(), nil
	}
	return true, "", "", map[string]any{"deleted": it.ID}
}
