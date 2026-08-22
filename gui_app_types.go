package main

import "time"

type localApplication struct {
	now func() time.Time
}

func newLocalApplication() *localApplication {
	return &localApplication{now: time.Now}
}

type machineInspection struct {
	Machine          machineFingerprintRecord  `json:"machine"`
	Models           int                       `json:"models"`
	EvidenceRuns     int                       `json:"evidence_runs"`
	Runtime          runtimeProjection         `json:"runtime"`
	LocalCTL         localctlFingerprintRecord `json:"localctl"`
	EvidenceRoot     string                    `json:"evidence_root,omitempty"`
	IntelligenceRoot string                    `json:"intelligence_root,omitempty"`
}

type runtimeProjection struct {
	State     string `json:"state"`
	Ready     bool   `json:"ready"`
	ModelID   string `json:"model_id,omitempty"`
	ProfileID string `json:"profile_id,omitempty"`
	PID       int    `json:"pid,omitempty"`
	URL       string `json:"url"`
}

type modelProjection struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Path              string            `json:"path"`
	SizeBytes         int64             `json:"size_bytes"`
	Size              string            `json:"size"`
	Quantization      string            `json:"quantization,omitempty"`
	Installed         bool              `json:"installed"`
	Tested            bool              `json:"tested"`
	Demonstrated      bool              `json:"demonstrated"`
	SavedRuns         int               `json:"saved_runs"`
	LatestRun         *time.Time        `json:"latest_run,omitempty"`
	Freshness         string            `json:"freshness"`
	ActiveRuntime     bool              `json:"active_runtime"`
	SelectedCandidate bool              `json:"selected_candidate"`
	CapabilityStates  map[string]string `json:"capability_states,omitempty"`
}

type territoryLeader struct {
	ModelID        string  `json:"model_id"`
	ModelName      string  `json:"model_name"`
	State          string  `json:"state"`
	Strength       string  `json:"strength"`
	Freshness      string  `json:"freshness"`
	ComparableRuns int     `json:"comparable_runs"`
	PassRate       float64 `json:"pass_rate"`
	Coverage       float64 `json:"coverage"`
}

type territoryCell struct {
	ID                 string           `json:"id"`
	Title              string           `json:"title"`
	Question           string           `json:"question"`
	PackID             string           `json:"pack_id,omitempty"`
	State              string           `json:"state"`
	Strength           string           `json:"strength"`
	Freshness          string           `json:"freshness"`
	ComparableRuns     int              `json:"comparable_runs"`
	EvidenceLeader     *territoryLeader `json:"evidence_leader,omitempty"`
	SourceRunIDs       []string         `json:"source_run_ids,omitempty"`
	RuleVersion        string           `json:"rule_version,omitempty"`
	PackVersion        string           `json:"pack_version,omitempty"`
	Limitations        []string         `json:"limitations,omitempty"`
	InvestigationReady bool             `json:"investigation_ready"`
}

type territoryProjection struct {
	GeneratedAt  time.Time                `json:"generated_at"`
	ProfileID    string                   `json:"profile_id"`
	Machine      machineFingerprintRecord `json:"machine"`
	Cells        []territoryCell          `json:"cells"`
	EvidenceRuns int                      `json:"evidence_runs"`
}

type missionRequirement struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	PackID    string `json:"pack_id,omitempty"`
	State     string `json:"state"`
	Freshness string `json:"freshness"`
	Satisfied bool   `json:"satisfied"`
	Runnable  bool   `json:"runnable"`
	Reason    string `json:"reason"`
}

type missionPlan struct {
	MissionID     string               `json:"mission_id"`
	Title         string               `json:"title"`
	Question      string               `json:"question"`
	ModelID       string               `json:"model_id"`
	ModelName     string               `json:"model_name"`
	ProfileID     string               `json:"profile_id"`
	Complete      bool                 `json:"complete"`
	Requirements  []missionRequirement `json:"requirements"`
	RunnablePacks []string             `json:"runnable_packs"`
	Unresolved    []string             `json:"unresolved,omitempty"`
}

type appMissionDefinition struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	Question            string   `json:"question"`
	Description         string   `json:"description"`
	PackIDs             []string `json:"pack_ids"`
	NonPackRequirements []string `json:"non_pack_requirements,omitempty"`
}

var appMissions = []appMissionDefinition{
	{ID: "developer", Title: "Local coding", Question: "Can I code locally?", Description: "Establish current coding, structured-output, and technical-writing evidence before treating a model as a local developer helper.", PackIDs: []string{"developer-core", "structured-output", "writing-summarization"}},
	{ID: "systems", Title: "Systems investigation", Question: "Which installed model is useful for systems troubleshooting?", Description: "Characterize Linux, Docker, and Kubernetes investigation without granting remediation authority.", PackIDs: []string{"linux-investigation", "docker-investigation", "kubernetes-investigation"}},
	{ID: "local-first", Title: "Move bounded work local", Question: "What useful work could reasonably move local?", Description: "Build evidence across structured output, developer work, writing, and bounded reasoning.", PackIDs: []string{"structured-output", "developer-core", "writing-summarization", "reasoning-analysis"}},
	{ID: "coding-agent", Title: "Coding-agent prerequisites", Question: "Can this machine run a useful coding agent?", Description: "Resolve the evidence LocalCTL can measure today, while keeping tool-use explicitly unknown until a real tool-use pack exists.", PackIDs: []string{"developer-core", "structured-output", "reasoning-analysis"}, NonPackRequirements: []string{"agent-tool-use"}},
}

type historyProjection struct {
	Sessions    []sessionRecord             `json:"sessions"`
	Experiments []experimentRecord          `json:"experiments"`
	Runs        []runObservation            `json:"runs"`
	Snapshots   []capabilitySnapshotSummary `json:"capability_snapshots"`
	Judgments   []humanJudgment             `json:"judgments"`
}

type runDetailProjection struct {
	Observation runObservation `json:"observation"`
	Judgment    *humanJudgment `json:"judgment,omitempty"`
	Prompt      string         `json:"prompt,omitempty"`
	Response    string         `json:"response,omitempty"`
}

type scoutItem struct {
	ID          string   `json:"id"`
	Priority    int      `json:"priority"`
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Why         string   `json:"why"`
	MissionID   string   `json:"mission_id,omitempty"`
	ModelID     string   `json:"model_id,omitempty"`
	PackID      string   `json:"pack_id,omitempty"`
	Limitations []string `json:"limitations,omitempty"`
	Runnable    bool     `json:"runnable"`
}

type useRecommendation struct {
	Workload     string                 `json:"workload"`
	Title        string                 `json:"title"`
	PackID       string                 `json:"pack_id"`
	State        string                 `json:"state"`
	Supported    bool                   `json:"supported"`
	Leader       *territoryLeader       `json:"leader,omitempty"`
	Profile      profileDefinition      `json:"profile"`
	EvidenceRuns int                    `json:"evidence_runs"`
	Limitations  []string               `json:"limitations,omitempty"`
	Preview      map[string]interface{} `json:"preview,omitempty"`
	SourceRunIDs []string               `json:"source_run_ids,omitempty"`
}

type settingsProjection struct {
	BindAddress        string              `json:"bind_address"`
	RuntimeURL         string              `json:"runtime_url"`
	EvidenceRoot       string              `json:"evidence_root"`
	IntelligenceRoot   string              `json:"intelligence_root"`
	JobsRoot           string              `json:"jobs_root"`
	Profiles           []profileDefinition `json:"profiles"`
	Security           []string            `json:"security"`
	ReadIndex          string              `json:"read_index"`
	ReadIndexAuthority string              `json:"read_index_authority"`
}

type territoryDefinition struct {
	ID          string
	Title       string
	Question    string
	PackID      string
	Limitations []string
}

var territoryDefinitions = []territoryDefinition{
	{ID: "coding", Title: "Coding", Question: "Can this machine handle focused software-development work?", PackID: "developer-core"},
	{ID: "structured-output", Title: "Structured output", Question: "Can a local model reliably honor bounded output contracts?", PackID: "structured-output"},
	{ID: "reasoning", Title: "Reasoning", Question: "What bounded reasoning and analysis has been demonstrated?", PackID: "reasoning-analysis"},
	{ID: "linux", Title: "Linux troubleshooting", Question: "Can a local model investigate Linux evidence usefully?", PackID: "linux-investigation"},
	{ID: "docker", Title: "Docker troubleshooting", Question: "Can a local model investigate Docker evidence usefully?", PackID: "docker-investigation"},
	{ID: "kubernetes", Title: "Kubernetes troubleshooting", Question: "Can a local model investigate Kubernetes evidence usefully?", PackID: "kubernetes-investigation"},
	{ID: "writing", Title: "Writing", Question: "Can a local model handle bounded technical writing and summarization?", PackID: "writing-summarization"},
	{ID: "long-context", Title: "Long context", Question: "How practical is long-context work on this machine?", Limitations: []string{"LocalCTL has a long-context runtime profile but no dedicated, versioned long-context capability pack yet. This territory remains UNKNOWN rather than borrowing unrelated pack results."}},
	{ID: "agent-tool-use", Title: "Agent / tool use", Question: "Can this machine support a useful local coding agent?", Limitations: []string{"Tool-use is not directly measured by the current canonical pack catalog. Coding and structured-output evidence are prerequisites, not proof of tool-use."}},
}
