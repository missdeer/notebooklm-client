package rpc

const (
	CreateNotebook       = "CCqFvf"
	ListNotebooks        = "wXbhsf"
	GetNotebook          = "rLM1Ne"
	RenameNotebook       = "s0tc2d"
	DeleteNotebook       = "WWINqb"
	RemoveRecentlyViewed = "fejl7e"

	AddSource        = "izAoDd"
	AddSourceFile    = "o4cbdc"
	GetSourceContent = "hizoJc"
	GetSourceSummary = "tr032e"
	DeleteSource     = "tGMBJ"
	RefreshSource    = "FLmJqe"
	UpdateSource     = "b7Wfje"

	CreateWebSearch    = "Ljjv0c"
	CreateDeepResearch = "QA9ei"
	PollResearch       = "e3bVqc"
	ImportResearch     = "LBwxtb"

	GenerateArtifact     = "R7cb6c"
	GetArtifactsFiltered = "gArtLc"
	DeleteArtifact       = "V5N4be"
	RenameArtifact       = "rc3d8d"
	GetInteractiveHTML   = "v9rmvd"
	ExportArtifact       = "Krh3pd"
	ShareArtifact        = "RGP97b"
	GetStudioConfig      = "sqTeoe"

	CreateNote = "CYK0Xb"
	GetNotes   = "cFji9"
	UpdateNote = "cYAfTb"
	DeleteNote = "AH0mwd"

	DeleteChatThread = "J7Gthc"

	GetShareStatus = "JFMDGd"
	ShareNotebook  = "QDyure"

	GetAccountInfo       = "ZwVcOc"
	SetUserSettings      = "hT54vc"
	GetNotebookSummary   = "VfAZjd"
	GetRecommendedTopics = "otmP3b"
	GetUIConfig          = "ozz5Z"
	ReportPlayProgress   = "Fxmvse"
)

const (
	ArtifactAudio       = 1
	ArtifactReport      = 2
	ArtifactVideo       = 3
	ArtifactQuiz        = 4
	ArtifactMindMap     = 5
	ArtifactInfographic = 7
	ArtifactSlideDeck   = 8
	ArtifactDataTable   = 9
)

const (
	BaseURL         = "https://notebooklm.google.com"
	DashboardURL    = "https://notebooklm.google.com/"
	BatchExecuteURL = "https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute"
	ChatStreamURL   = "https://notebooklm.google.com/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed"
	UploadURL       = "https://notebooklm.google.com/upload/_/"
)

// DefaultUserConfig is the static user configuration payload fragment.
var DefaultUserConfig = []any{2, nil, nil, []any{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []any{1}}, []any{[]any{2, 1, 3}}}

// PlatformWeb identifies the web platform in RPC payloads.
var PlatformWeb = []any{2}
