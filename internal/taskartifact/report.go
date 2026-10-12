package taskartifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Service) Enqueue(id string, retry bool) error {
	state, e := s.State(id)
	if e != nil {
		return e
	}
	if !state.Options.Report.Enabled || state.Artifacts.Report.State == "unavailable" {
		return fmt.Errorf("本次执行未开启报告")
	}
	task, e := s.database.GetTaskByID(id)
	if e != nil {
		return e
	}
	if task == nil || task.EndedAt == 0 || state.Artifacts.Log.CaptureState == "running" || state.Artifacts.Log.CaptureState == "waiting" {
		return fmt.Errorf("执行尚未结束")
	}
	previous, e := s.database.Report(id, 0, false)
	if e != nil {
		return e
	}
	evidence, prompt, hash := "", state.Options.Report.Prompt, ""
	if previous != nil {
		if !retry {
			return nil
		}
		evidence, prompt, hash = previous.Evidence, previous.Prompt, previous.InputHash
	} else {
		snapshot := Snapshot{}
		if state.DispatchID != "" {
			if e = s.database.DispatchSnapshot(state.DispatchID, &snapshot); e != nil {
				return e
			}
		}
		ev := Evidence{TaskID: id, DeviceSerial: task.DeviceSerial, Status: task.Status, ExitCode: task.ExitCode, ErrorMsg: task.ErrorMsg, StartedAt: task.StartedAt, EndedAt: task.EndedAt, DocumentHash: snapshot.DocumentHash, DocumentVersion: snapshot.DocumentVersion, Recording: state.Artifacts.Recording, Log: state.Artifacts.Log}
		lines, e := s.selectEvidence(id)
		if e != nil {
			return e
		}
		if len(ev.ErrorMsg) > 2048 {
			ev.ErrorMsg = ev.ErrorMsg[:2048]
			for !utf8.ValidString(ev.ErrorMsg) {
				ev.ErrorMsg = ev.ErrorMsg[:len(ev.ErrorMsg)-1]
			}
			ev.MetadataTruncated = true
		}
		ev.Lines = lines
		for _, line := range lines {
			b, _ := json.Marshal(line)
			ev.SelectedBytes += len(b)
		}
		ev.SelectionTruncated = int64(len(lines)) < state.Artifacts.Log.LastSeq
		b, _ := json.Marshal(ev)
		for len(b) > 64<<10 && len(ev.Lines) > 0 {
			removed, _ := json.Marshal(ev.Lines[len(ev.Lines)-1])
			ev.SelectedBytes -= len(removed)
			ev.Lines = ev.Lines[:len(ev.Lines)-1]
			ev.SelectionTruncated = true
			b, _ = json.Marshal(ev)
		}
		if len(b) > 64<<10 {
			return fmt.Errorf("报告执行信息超出输入上限")
		}
		evidence = string(b)
		h := sha256.Sum256(b)
		hash = hex.EncodeToString(h[:])
	}
	if _, e = s.database.EnqueueReport(id, evidence, prompt, hash); e != nil {
		return e
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return nil
}

// Select head/tail and stderr/system deterministically within the evidence budget.
func (s *Service) selectEvidence(id string) ([]LogLine, error) {
	if s.Logs == nil {
		return []LogLine{}, nil
	}
	f, e := s.Logs.Open(id)
	if e != nil {
		return []LogLine{}, nil
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	head := []LogLine{}
	tail := []LogLine{}
	priority := []LogLine{}
	priorityBytes := 0
	for decoder.More() {
		var line LogLine
		if e := decoder.Decode(&line); e != nil {
			return nil, e
		}
		b, _ := json.Marshal(line)
		if len(head) < 8 {
			head = append(head, line)
		}
		tail = append(tail, line)
		if len(tail) > 32 {
			tail = tail[1:]
		}
		if (line.Stream == "system" || line.Stream == "stderr") && priorityBytes+len(b) <= 24<<10 {
			priority = append(priority, line)
			priorityBytes += len(b)
		}
	}
	selected := map[int64]LogLine{}
	size := 0
	for _, group := range [][]LogLine{priority, head, tail} {
		for _, line := range group {
			if _, ok := selected[line.Seq]; ok {
				continue
			}
			b, _ := json.Marshal(line)
			if size+len(b) > 60<<10 {
				continue
			}
			selected[line.Seq] = line
			size += len(b)
		}
	}
	lines := []LogLine{}
	for _, line := range selected {
		lines = append(lines, line)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].Seq < lines[j].Seq })
	return lines, nil
}
func (s *Service) reportWorker(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	cleanup := time.NewTicker(time.Minute)
	defer cleanup.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		job, e := s.database.ClaimReport()
		if e != nil {
			log.Printf("[TaskReport] 读取队列失败: %v", e)
		}
		if job != nil {
			callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			out, callErr := s.model(callCtx, job.Prompt, job.Evidence)
			cancel()
			if callErr == nil {
				var ev Evidence
				if e = json.Unmarshal([]byte(job.Evidence), &ev); e != nil {
					callErr = e
				} else {
					valid := map[int64]bool{}
					for _, line := range ev.Lines {
						valid[line.Seq] = true
					}
					for _, c := range out.Citations {
						if !valid[c.Seq] {
							callErr = fmt.Errorf("报告引用了未提供的日志行 %d", c.Seq)
							break
						}
					}
				}
				if strings.TrimSpace(out.Markdown) == "" {
					callErr = fmt.Errorf("模型返回空报告")
				}
			}
			output := ""
			message := ""
			if callErr != nil {
				message = callErr.Error()
			} else {
				b, _ := json.Marshal(out)
				output = string(b)
			}
			if e = s.database.FinishReport(job.TaskID, job.Revision, output, message); e != nil {
				log.Printf("[TaskReport] task=%s revision=%d 保存失败: %v", job.TaskID, job.Revision, e)
			}
			log.Printf("[TaskReport] task=%s revision=%d finished success=%t", job.TaskID, job.Revision, callErr == nil)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
		case <-cleanup.C:
			s.Logs.Cleanup()
		}
	}
}
