package diagram

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// ReachDirection defines traversal direction.
type ReachDirection string

const (
	DirectionUpstream   ReachDirection = "upstream"   // nodes this node depends on (incoming edges)
	DirectionDownstream ReachDirection = "downstream" // nodes that depend on this node (outgoing edges)
	DirectionBoth       ReachDirection = "both"
)

// ReachResult represents graph reachability from a given starting component.
type ReachResult struct {
	TargetNode     string   `json:"target_node"`
	Direction      string   `json:"direction"`
	ReachableNodes []string `json:"reachable_nodes"`
	ReachableCount int      `json:"reachable_count"`
	Path           []string `json:"path,omitempty"`
	Summary        string   `json:"summary"`
}

// ReachFile runs reach analysis from a file.
func ReachFile(filePath string, fromNode string, direction ReachDirection) (*ReachResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read diagram file: %w", err)
	}
	return Reach(data, fromNode, direction)
}

// Reach performs BFS traversal to determine reachable nodes in the specified direction.
func Reach(data []byte, fromNode string, direction ReachDirection) (*ReachResult, error) {
	var spec DiagramSpecification
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("unmarshal diagram specification: %w", err)
	}

	nodeExists := false
	for _, c := range spec.Components {
		if c.ID == fromNode {
			nodeExists = true
			break
		}
	}
	if !nodeExists {
		return nil, fmt.Errorf("target component %q does not exist in diagram", fromNode)
	}

	outgoing := make(map[string][]string)
	incoming := make(map[string][]string)

	for _, conn := range spec.Connections {
		outgoing[conn.From] = append(outgoing[conn.From], conn.To)
		incoming[conn.To] = append(incoming[conn.To], conn.From)
	}

	visited := make(map[string]bool)
	queue := []string{fromNode}
	visited[fromNode] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		var neighbors []string
		if direction == DirectionDownstream || direction == DirectionBoth {
			neighbors = append(neighbors, outgoing[curr]...)
		}
		if direction == DirectionUpstream || direction == DirectionBoth {
			neighbors = append(neighbors, incoming[curr]...)
		}

		for _, next := range neighbors {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	var reachable []string
	for k := range visited {
		if k != fromNode {
			reachable = append(reachable, k)
		}
	}
	sort.Strings(reachable)

	dirStr := string(direction)
	if dirStr == "" {
		dirStr = string(DirectionDownstream)
	}

	return &ReachResult{
		TargetNode:     fromNode,
		Direction:      dirStr,
		ReachableNodes: reachable,
		ReachableCount: len(reachable),
		Summary:        fmt.Sprintf("Component %q reaches %d nodes (%s)", fromNode, len(reachable), dirStr),
	}, nil
}

// ShortestPath finds the shortest directed route between source and destination components.
func ShortestPath(data []byte, from, to string) ([]string, error) {
	var spec DiagramSpecification
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}

	outgoing := make(map[string][]string)
	for _, conn := range spec.Connections {
		outgoing[conn.From] = append(outgoing[conn.From], conn.To)
	}

	prev := make(map[string]string)
	visited := make(map[string]bool)
	queue := []string{from}
	visited[from] = true

	found := false
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr == to {
			found = true
			break
		}

		for _, next := range outgoing[curr] {
			if !visited[next] {
				visited[next] = true
				prev[next] = curr
				queue = append(queue, next)
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("no path found between %q and %q", from, to)
	}

	path := []string{to}
	step := to
	for step != from {
		step = prev[step]
		path = append([]string{step}, path...)
	}

	return path, nil
}
