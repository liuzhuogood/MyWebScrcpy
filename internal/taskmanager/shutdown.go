package taskmanager

import "log"

// Close ends only processes owned by this manager; stored PIDs from other
// sessions are not touched. Call after stopping the scheduler.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	ids := make([]string, 0, len(m.procs))
	for id := range m.procs {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		if err := m.KillTask(id); err != nil {
			log.Printf("[TaskManager] 退出清理 task=%s 失败: %v", id, err)
		}
	}
	m.workers.Wait()
}
