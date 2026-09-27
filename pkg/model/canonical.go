package model

// ProcessModel represents the top-level canonical graph of analyzed workflows.
type ProcessModel struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Swimlanes []Swimlane     `json:"swimlanes"`
	Steps     []Step         `json:"steps"`
	Links     []Link         `json:"links"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// ASTNode represents an abstract syntax tree node for code analysis.
type ASTNode struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	Children []*ASTNode     `json:"children,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

// Step represents a discreet executable action or endpoint within a workflow.
type Step struct {
	ID          string         `json:"id"`
	SwimlaneID  string         `json:"swimlane_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Type        string         `json:"type"` // e.g., "Endpoint", "Function", "DatabaseQuery", "WorkflowStep", "Task"
	Language    string         `json:"language"`
	SourceFile  string         `json:"source_file"`
	LineNumber  int            `json:"line_number"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Link represents a directed connection or transition between two Steps.
type Link struct {
	ID             string `json:"id"`
	SourceStepID   string `json:"source_step_id"`
	TargetStepID   string `json:"target_step_id"`
	Label          string `json:"label,omitempty"`
	Condition      string `json:"condition,omitempty"`
	IsCrossService bool   `json:"is_cross_service"`
}

// Swimlane categorizes steps by service, domain, module, or layer.
type Swimlane struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// NewProcessModel initializes an empty ProcessModel.
func NewProcessModel(id, name string) *ProcessModel {
	return &ProcessModel{
		ID:        id,
		Name:      name,
		Swimlanes: make([]Swimlane, 0),
		Steps:     make([]Step, 0),
		Links:     make([]Link, 0),
		Metadata:  make(map[string]any),
	}
}

// AddSwimlane adds a new swimlane if not already present.
func (pm *ProcessModel) AddSwimlane(swimlane Swimlane) {
	for _, s := range pm.Swimlanes {
		if s.ID == swimlane.ID {
			return
		}
	}
	pm.Swimlanes = append(pm.Swimlanes, swimlane)
}

// AddStep appends a step to the model.
func (pm *ProcessModel) AddStep(step Step) {
	for i, s := range pm.Steps {
		if s.ID == step.ID {
			pm.Steps[i] = step
			return
		}
	}
	pm.Steps = append(pm.Steps, step)
}

// AddLink appends a link between steps.
func (pm *ProcessModel) AddLink(link Link) {
	for _, l := range pm.Links {
		if l.SourceStepID == link.SourceStepID && l.TargetStepID == link.TargetStepID && l.Label == link.Label {
			return
		}
	}
	pm.Links = append(pm.Links, link)
}

// FindStep finds a step by ID.
func (pm *ProcessModel) FindStep(id string) *Step {
	for i := range pm.Steps {
		if pm.Steps[i].ID == id {
			return &pm.Steps[i]
		}
	}
	return nil
}

// FindSwimlane finds a swimlane by ID.
func (pm *ProcessModel) FindSwimlane(id string) *Swimlane {
	for i := range pm.Swimlanes {
		if pm.Swimlanes[i].ID == id {
			return &pm.Swimlanes[i]
		}
	}
	return nil
}
