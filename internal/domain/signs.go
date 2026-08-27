package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type SignValidationError struct {
	SignID  string `json:"sign_id,omitempty"`
	Index   int    `json:"index"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SignDiff struct {
	Added     []string `json:"added"`
	Modified  []string `json:"modified"`
	Deleted   []string `json:"deleted"`
	Unchanged []string `json:"unchanged"`
}

type SignPreflight struct {
	Summary string                `json:"summary"`
	Diff    SignDiff              `json:"diff"`
	Errors  []SignValidationError `json:"errors"`
	Signs   []SignProposal        `json:"signs"`
}

var allowedDirections = map[string]bool{"straight": true, "forward": true, "left": true, "right": true, "back": true, "u_turn": true, "up": true, "down": true, "arrive": true}

func NormalizeSigns(signs []SignProposal) []SignProposal {
	out := append([]SignProposal{}, signs...)
	for i := range out {
		s := &out[i]
		s.SignID = strings.TrimSpace(s.SignID)
		s.NodeID = strings.TrimSpace(s.NodeID)
		s.Direction = strings.ToLower(strings.TrimSpace(s.Direction))
		s.DisplayText = strings.Join(strings.Fields(s.DisplayText), " ")
		s.RevisionNote = strings.Join(strings.Fields(s.RevisionNote), " ")
		for j := range s.DestinationRefs {
			s.DestinationRefs[j] = strings.TrimSpace(s.DestinationRefs[j])
		}
		s.DestinationRefs = uniqueSorted(s.DestinationRefs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SignID < out[j].SignID })
	return out
}

func PreflightSigns(graph SurveyGraph, current, candidate []SignProposal) SignPreflight {
	normalized := NormalizeSigns(candidate)
	nodes, destinations := map[string]bool{}, map[string]bool{}
	for _, n := range graph.Nodes {
		nodes[n.ID] = true
	}
	for _, id := range graph.DestinationIDs {
		destinations[id] = true
	}
	errs, seen := make([]SignValidationError, 0), map[string]bool{}
	add := func(i int, s SignProposal, field, code, message string) {
		errs = append(errs, SignValidationError{s.SignID, i, field, code, message})
	}
	for i, s := range normalized {
		if s.SignID == "" {
			add(i, s, "sign_id", "EMPTY_SIGN_ID", "sign_id 不能为空")
		} else if seen[s.SignID] {
			add(i, s, "sign_id", "DUPLICATE_SIGN_ID", fmt.Sprintf("重复 sign_id: %s", s.SignID))
		}
		seen[s.SignID] = true
		if !nodes[s.NodeID] {
			add(i, s, "node_id", "UNKNOWN_NODE", fmt.Sprintf("标识 %s 引用未知节点 %s", s.SignID, s.NodeID))
		}
		if len(s.DestinationRefs) == 0 {
			add(i, s, "destination_refs", "EMPTY_DESTINATIONS", fmt.Sprintf("标识 %s 必须引用目的地", s.SignID))
		}
		for _, id := range s.DestinationRefs {
			if !destinations[id] {
				add(i, s, "destination_refs", "UNKNOWN_DESTINATION", fmt.Sprintf("标识 %s 引用未知目的地 %s", s.SignID, id))
			}
		}
		if !allowedDirections[s.Direction] {
			add(i, s, "direction", "UNKNOWN_DIRECTION", fmt.Sprintf("标识 %s 的方向值无效", s.SignID))
		}
		if strings.TrimSpace(s.DisplayText) == "" {
			add(i, s, "display_text", "EMPTY_DISPLAY_TEXT", fmt.Sprintf("标识 %s 的显示内容不能为空", s.SignID))
		}
		if math.IsNaN(s.VisibilityDistanceM) || math.IsInf(s.VisibilityDistanceM, 0) || s.VisibilityDistanceM <= 0 {
			add(i, s, "visibility_distance_m", "INVALID_VISIBILITY_DISTANCE", fmt.Sprintf("标识 %s 的可视距离必须是有限正数", s.SignID))
		}
	}
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].SignID != errs[j].SignID {
			return errs[i].SignID < errs[j].SignID
		}
		if errs[i].Field != errs[j].Field {
			return errs[i].Field < errs[j].Field
		}
		return errs[i].Code < errs[j].Code
	})
	diff := diffSigns(NormalizeSigns(current), normalized)
	result := SignPreflight{Diff: diff, Errors: errs, Signs: normalized}
	result.Summary = Digest(struct {
		Signs  []SignProposal        `json:"signs"`
		Diff   SignDiff              `json:"diff"`
		Errors []SignValidationError `json:"errors"`
	}{normalized, diff, errs})
	return result
}

func diffSigns(current, candidate []SignProposal) SignDiff {
	a, b := map[string]SignProposal{}, map[string]SignProposal{}
	for _, s := range current {
		a[s.SignID] = s
	}
	for _, s := range candidate {
		b[s.SignID] = s
	}
	d := SignDiff{}
	for id, next := range b {
		old, ok := a[id]
		if !ok {
			d.Added = append(d.Added, id)
		} else if Digest(old) != Digest(next) {
			d.Modified = append(d.Modified, id)
		} else {
			d.Unchanged = append(d.Unchanged, id)
		}
	}
	for id := range a {
		if _, ok := b[id]; !ok {
			d.Deleted = append(d.Deleted, id)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Modified)
	sort.Strings(d.Deleted)
	sort.Strings(d.Unchanged)
	return d
}

func ChangedSignScope(current, candidate []SignProposal) (map[string]bool, map[string]bool) {
	pre := PreflightSigns(SurveyGraph{}, current, candidate)
	changed := append(append(append([]string{}, pre.Diff.Added...), pre.Diff.Modified...), pre.Diff.Deleted...)
	all := append(NormalizeSigns(current), NormalizeSigns(candidate)...)
	nodes, destinations := map[string]bool{}, map[string]bool{}
	wanted := map[string]bool{}
	for _, id := range changed {
		wanted[id] = true
	}
	for _, s := range all {
		if wanted[s.SignID] {
			nodes[s.NodeID] = true
			for _, d := range s.DestinationRefs {
				destinations[d] = true
			}
		}
	}
	return nodes, destinations
}
