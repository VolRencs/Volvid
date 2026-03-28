package bot

import (
	"sort"
	"strings"
)

func loadIDSetFile(path string) (map[int64]struct{}, error) {
	var ids []int64
	if err := loadJSONFile(path, []int64{}, &ids); err != nil {
		return nil, err
	}
	return idSliceToSet(ids), nil
}

func loadTimerEntryMapFile(path string) (map[string]TimerEntry, error) {
	var entries []TimerEntry
	if err := loadJSONFile(path, []TimerEntry{}, &entries); err != nil {
		return nil, err
	}
	return timerEntryMap(entries), nil
}

func sortedIDSet(ids map[int64]struct{}) []int64 {
	out := make([]int64, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func idSliceToSet(ids []int64) map[int64]struct{} {
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func sortedTimerEntries(items map[string]TimerEntry) []TimerEntry {
	out := make([]TimerEntry, 0, len(items))
	for _, entry := range items {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NextRun.Equal(out[j].NextRun) {
			return out[i].ID < out[j].ID
		}
		return out[i].NextRun.Before(out[j].NextRun)
	})
	return out
}

func timerEntryMap(entries []TimerEntry) map[string]TimerEntry {
	out := make(map[string]TimerEntry, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" {
			continue
		}
		out[entry.ID] = entry
	}
	return out
}
