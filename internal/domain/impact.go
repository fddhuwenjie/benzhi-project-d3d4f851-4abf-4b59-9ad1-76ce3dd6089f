package domain

import "sort"

type IssueFilter struct {
	Severity string `json:"severity,omitempty"`
	RuleCode string `json:"rule_code,omitempty"`
	NodeID   string `json:"node_id,omitempty"`
	Status   string `json:"status,omitempty"`
}

type IssueAggregate struct {
	Severity        string `json:"severity"`
	RuleCode        string `json:"rule_code"`
	NodeID          string `json:"node_id"`
	UnresolvedCount int    `json:"unresolved_count"`
}

type IssueSummary struct {
	UnresolvedCount          int              `json:"unresolved_count"`
	AffectedDestinationCount int              `json:"affected_destination_count"`
	Aggregates               []IssueAggregate `json:"aggregates"`
}

func AnalyzeIssueImpact(g SurveyGraph, issue ValidationIssue) IssueImpact {
	g = NormalizeGraph(g)
	impact := IssueImpact{EntranceIDs: []string{}, DestinationIDs: []string{}, Paths: [][]string{}}
	if issue.RuleCode == RuleDestinationReachability {
		impact.DestinationIDs = []string{issue.NodeID}
		impact.CheckedEntrances = append([]string{}, g.EntranceIDs...)
		sort.Strings(impact.CheckedEntrances)
		return impact
	}
	for _, entrance := range g.EntranceIDs {
		toIssue := deterministicPath(g, entrance, issue.NodeID)
		if len(toIssue) == 0 {
			continue
		}
		for _, destination := range g.DestinationIDs {
			fromIssue := deterministicPath(g, issue.NodeID, destination)
			if len(fromIssue) > 0 {
				path := append(append([]string{}, toIssue...), fromIssue[1:]...)
				impact.EntranceIDs = append(impact.EntranceIDs, entrance)
				impact.DestinationIDs = append(impact.DestinationIDs, destination)
				impact.Paths = append(impact.Paths, path)
			}
		}
	}
	impact.EntranceIDs = uniqueSorted(impact.EntranceIDs)
	impact.DestinationIDs = uniqueSorted(impact.DestinationIDs)
	sort.Slice(impact.Paths, func(i, j int) bool { return joinPath(impact.Paths[i]) < joinPath(impact.Paths[j]) })
	return impact
}

func deterministicPath(g SurveyGraph, from, to string) []string {
	adj := map[string][]string{}
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	for id := range adj {
		sort.Strings(adj[id])
	}
	q := [][]string{{from}}
	seen := map[string]bool{from: true}
	for len(q) > 0 {
		path := q[0]
		q = q[1:]
		last := path[len(path)-1]
		if last == to {
			return path
		}
		for _, next := range adj[last] {
			if !seen[next] {
				seen[next] = true
				cp := append([]string{}, path...)
				q = append(q, append(cp, next))
			}
		}
	}
	return nil
}

func joinPath(path []string) string {
	out := ""
	for _, x := range path {
		out += "\x00" + x
	}
	return out
}

func QueryIssues(issues []ValidationIssue, filter IssueFilter) ([]ValidationIssue, IssueSummary) {
	out := make([]ValidationIssue, 0)
	for _, issue := range issues {
		if filter.Severity != "" && issue.Severity != filter.Severity {
			continue
		}
		if filter.RuleCode != "" && issue.RuleCode != filter.RuleCode {
			continue
		}
		if filter.NodeID != "" && issue.NodeID != filter.NodeID {
			continue
		}
		if filter.Status != "" && string(issue.Status) != filter.Status {
			continue
		}
		out = append(out, issue)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity < out[j].Severity
		}
		if out[i].RuleCode != out[j].RuleCode {
			return out[i].RuleCode < out[j].RuleCode
		}
		if out[i].NodeID != out[j].NodeID {
			return out[i].NodeID < out[j].NodeID
		}
		return out[i].IssueID < out[j].IssueID
	})
	summary := IssueSummary{}
	destinations := map[string]bool{}
	aggregate := map[string]*IssueAggregate{}
	for _, issue := range out {
		if issue.Status != IssueOpen {
			continue
		}
		summary.UnresolvedCount++
		for _, id := range issue.Impact.DestinationIDs {
			destinations[id] = true
		}
		key := issue.Severity + "\x00" + issue.RuleCode + "\x00" + issue.NodeID
		if aggregate[key] == nil {
			aggregate[key] = &IssueAggregate{Severity: issue.Severity, RuleCode: issue.RuleCode, NodeID: issue.NodeID}
		}
		aggregate[key].UnresolvedCount++
	}
	summary.AffectedDestinationCount = len(destinations)
	for _, x := range aggregate {
		summary.Aggregates = append(summary.Aggregates, *x)
	}
	sort.Slice(summary.Aggregates, func(i, j int) bool {
		a, b := summary.Aggregates[i], summary.Aggregates[j]
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		if a.RuleCode != b.RuleCode {
			return a.RuleCode < b.RuleCode
		}
		return a.NodeID < b.NodeID
	})
	return out, summary
}
