package taskmanager

import (
	"fmt"
	"log"
	"mywebscrcpy/internal/taskartifact"
	"os"
)

// Recover verifies the unique temporary script path before terminating a surviving PID.
func (m *Manager) Recover() error {
	tasks, e := m.database.ListActiveTasks()
	if e != nil {
		return e
	}
	for _, task := range tasks {
		var st taskartifact.State
		found, e := m.database.ArtifactState(task.ID, &st)
		if e != nil {
			return e
		}
		if found && task.PID > 0 && st.ProcessPath != "" {
			owned, e := storedProcessOwnsPath(task.PID, st.ProcessPath)
			if e != nil {
				return fmt.Errorf("确认中断任务 %s 的进程失败: %w", task.ID, e)
			}
			if owned {
				terminateStoredTaskProcess(task.PID)
				log.Printf("[TaskManager] task=%s 清理重启前遗留的进程树", task.ID)
			}
			os.Remove(st.ProcessPath)
		}
	}
	return m.Artifacts.Recover()
}
