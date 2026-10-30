package manual

import (
	"fmt"
	"regexp"
	"strings"
)

// Tree is a decision tree read from a Mermaid flowchart. Convention (the
// diagnostic diagram, 12, follows it): a node written id{"…"} is a question,
// the labels of its outgoing arrows are the answers, and any other node is a
// step; a step's unlabeled arrow leads to the next step. The start is the only
// node without incoming arrows.
type Tree struct {
	Start string          `json:"start"`
	Nodes map[string]Node `json:"nodes"`
}

// Node is a question or a step.
type Node struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Question bool     `json:"question"`
	Answers  []Answer `json:"answers,omitempty"` // question
	Next     string   `json:"next,omitempty"`    // step: the following step, if any
}

// Answer leads from a question to a node.
type Answer struct {
	Label string `json:"label"`
	To    string `json:"to"`
}

var (
	reNodeDef = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*(\["([^"]*)"\]|\{"([^"]*)"\}|\(\["([^"]*)"\]\)|\[\("([^"]*)"\)\]|\[\["([^"]*)"\]\])`)
	reEdge    = regexp.MustCompile(`^(.+?)\s*(?:--\s*"([^"]*)"\s*-->|-\.\s*"([^"]*)"\s*\.->|-->|-\.->|==>)\s*(.+)$`)
)

// ParseTree reads the decision tree of a flowchart.
func ParseTree(mermaid string) (Tree, error) {
	t := Tree{Nodes: map[string]Node{}}
	incoming := map[string]bool{}
	var order []string
	text := func(s string) string {
		s = strings.ReplaceAll(s, "<br/>", " ")
		return strings.Join(strings.Fields(s), " ")
	}
	// Node definitions anywhere in the block.
	for _, m := range reNodeDef.FindAllStringSubmatch(mermaid, -1) {
		id := m[1]
		if _, ok := t.Nodes[id]; ok {
			continue
		}
		label := m[3] + m[4] + m[5] + m[6] + m[7]
		t.Nodes[id] = Node{ID: id, Text: text(label), Question: m[4] != ""}
		order = append(order, id)
	}
	// Edges: "a -- "label" --> b", "a --> b", "a & b --> c", chains "a --> b --> c".
	for _, raw := range strings.Split(mermaid, "\n") {
		line := reNodeDef.ReplaceAllString(strings.TrimSpace(raw), "$1")
		for {
			e := reEdge.FindStringSubmatch(line)
			if e == nil || strings.HasPrefix(line, "class") || strings.HasPrefix(line, "subgraph") {
				break
			}
			from, label := strings.TrimSpace(e[1]), e[2]+e[3]
			rest := strings.TrimSpace(e[4])
			to := rest
			if nxt := reEdge.FindStringSubmatchIndex(rest); nxt != nil {
				to = strings.TrimSpace(rest[:nxt[3]]) // the next edge's left side
				to = strings.Fields(to)[0]
			}
			for _, f := range strings.Split(from, "&") {
				f = strings.TrimSpace(f)
				for _, d := range strings.Split(to, "&") {
					d = strings.TrimSpace(d)
					n, ok := t.Nodes[f]
					if !ok {
						return Tree{}, fmt.Errorf("tree: arrow from undefined node %q", f)
					}
					if _, ok := t.Nodes[d]; !ok {
						return Tree{}, fmt.Errorf("tree: arrow to undefined node %q", d)
					}
					incoming[d] = true
					switch {
					case n.Question:
						if label == "" {
							return Tree{}, fmt.Errorf("tree: question %q has an arrow without an answer", f)
						}
						n.Answers = append(n.Answers, Answer{Label: text(label), To: d})
					case n.Next == "":
						n.Next = d
					default:
						return Tree{}, fmt.Errorf("tree: step %q has more than one next step", f)
					}
					t.Nodes[f] = n
				}
			}
			if rest == to {
				break
			}
			line = rest
		}
	}
	for _, id := range order {
		if !incoming[id] {
			if t.Start != "" {
				return Tree{}, fmt.Errorf("tree: two starts, %q and %q", t.Start, id)
			}
			t.Start = id
		}
	}
	if t.Start == "" {
		return Tree{}, fmt.Errorf("tree: no start node")
	}
	for id, n := range t.Nodes {
		if n.Question && len(n.Answers) < 2 {
			return Tree{}, fmt.Errorf("tree: question %q needs at least two answers", id)
		}
	}
	return t, nil
}
