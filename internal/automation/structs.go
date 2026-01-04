package automation

// ReactFlowNode represents a node in React-Flow
type ReactFlowNode struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Position map[string]float64 `json:"position"`
	Data     ReactFlowNodeData  `json:"data"`
}

// ReactFlowNodeData represents the data property of a node
type ReactFlowNodeData struct {
	Label   string          `json:"label"`
	Steps   []ReactFlowStep `json:"steps"`
	IsStart bool            `json:"isStart"`
}

// StepValidation holds validation rules for an input step
type StepValidation struct {
	MaxRetries   interface{} `json:"maxRetries"` // Can be string or int from JSON
	ErrorMessage string      `json:"errorMessage"`
	Regex        string      `json:"regex"`
	Min          interface{} `json:"min"`
	Max          interface{} `json:"max"`
}

// ReactFlowStep represents a step within a node (e.g. Text, Image)
type ReactFlowStep struct {
	Type       string          `json:"type"`
	Content    string          `json:"content"`
	Variable   string          `json:"variable"` // For saving input
	Buttons    []QuickReplyBtn `json:"buttons,omitempty"`
	Validation *StepValidation `json:"validation,omitempty"`
}

type QuickReplyBtn struct {
	Label string `json:"label"`
}

// ReactFlowEdge represents an edge connection
type ReactFlowEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"sourceHandle"`
}

// FlowGraphData represents the stored JSON in database
type FlowGraphData struct {
	Nodes []ReactFlowNode `json:"nodes"`
	Edges []ReactFlowEdge `json:"edges"`
}
