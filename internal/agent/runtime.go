package agent

func (m *Manager) setRuntime(runtime *runRuntime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runtimes[runtime.runID] = runtime
}

func (m *Manager) runtime(runID string) *runRuntime {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runtimes[runID]
}

func (m *Manager) deleteRuntime(runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.runtimes, runID)
}

func (m *Manager) appendDraftText(runID, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime := m.runtimes[runID]
	if runtime == nil {
		return
	}
	runtime.draftText.WriteString(text)
}

func (m *Manager) resetDraftText(runtime *runRuntime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime.draftText.Reset()
}

func (m *Manager) addRuntimeCommand(runtime *runRuntime, commandID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime.activeCommands[commandID] = struct{}{}
}

func (m *Manager) removeRuntimeCommand(runtime *runRuntime, commandID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(runtime.activeCommands, commandID)
}

func (m *Manager) stopRuntimeCommands(runtime *runRuntime) {
	m.mu.Lock()
	commandIDs := make([]string, 0, len(runtime.activeCommands))
	for commandID := range runtime.activeCommands {
		commandIDs = append(commandIDs, commandID)
	}
	m.mu.Unlock()
	for _, commandID := range commandIDs {
		m.runner.Stop(commandID)
	}
}
