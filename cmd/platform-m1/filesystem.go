package main

import "path/filepath"

// installDir returns the installation directory path.
func (p *manager) installDir() string {
	return filepath.Join(p.baseDir, "bin")
}

// initDir returns the init scripts directory path.
func (p *manager) initDir() string {
	return filepath.Join(p.baseDir, "init")
}

// etcDir returns the configuration files directory path.
func (p *manager) etcDir() string {
	return filepath.Join(p.baseDir, "etc")
}

// dataDir returns the data directory path.
func (p *manager) dataDir() string {
	return filepath.Join(p.baseDir, "data")
}

// updateDir returns the update directory path.
func (p *manager) updateDir() string {
	prefix := p.dataDir()

	return filepath.Join(prefix, "update")
}

// stateFile returns the path to the update state file.
func (p *manager) stateFile() string {
	prefix := p.updateDir()

	return filepath.Join(prefix, "update-state.json")
}

// rollbackArchive returns the path to the rollback archive file.
func (p *manager) rollbackArchive() string {
	prefix := p.updateDir()

	return filepath.Join(prefix, "rollback.tgz")
}

// failedStartCounter returns the path to the rescfailed start counter file used by recover.sh.
func (p *manager) failedStartCounter() string {
	prefix := p.dataDir()

	return filepath.Join(prefix, "failed_start.counter")
}

// stagingDir returns the path to the staging directory used during atomic updates.
// Original files are moved here before new files are installed.
func (p *manager) stagingDir() string {
	prefix := p.updateDir()

	return filepath.Join(prefix, "staging")
}
