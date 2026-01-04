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
	Type         string          `json:"type"`
	Content      string          `json:"content"`
	Variable     string          `json:"variable"` // For saving input
	Buttons      []QuickReplyBtn `json:"buttons,omitempty"`
	Options      []ListOption    `json:"options,omitempty"`    // For List messages
	ButtonText   string          `json:"buttonText,omitempty"` // For List button text
	Validation   *StepValidation `json:"validation,omitempty"`
	TargetFlowId string          `json:"targetFlowId,omitempty"` // For Chatbot step
	TargetNodeId string          `json:"targetNodeId,omitempty"` // For Chatbot step
	MediaId      string          `json:"mediaId,omitempty"`      // For Image, Video, Audio, File
	Url          string          `json:"url,omitempty"`          // For YouTube
	Latitude     string          `json:"latitude,omitempty"`     // For Location
	Longitude    string          `json:"longitude,omitempty"`    // For Location
	Name         string          `json:"name,omitempty"`         // For Location
	Address      string          `json:"address,omitempty"`      // For Location
}

type QuickReplyBtn struct {
	Label string `json:"label"`
}

type ListOption struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
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
