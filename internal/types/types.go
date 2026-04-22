package types

type SourceType string

const (
	SourceURL      SourceType = "url"
	SourceText     SourceType = "text"
	SourceResearch SourceType = "research"
	SourceFile     SourceType = "file"
)

type ResearchMode string

const (
	ResearchFast ResearchMode = "fast"
	ResearchDeep ResearchMode = "deep"
)

type AudioLanguage string

const (
	LangEN AudioLanguage = "en"
	LangZH AudioLanguage = "zh"
	LangJA AudioLanguage = "ja"
	LangKO AudioLanguage = "ko"
	LangES AudioLanguage = "es"
	LangFR AudioLanguage = "fr"
	LangDE AudioLanguage = "de"
	LangPT AudioLanguage = "pt"
	LangIT AudioLanguage = "it"
	LangHI AudioLanguage = "hi"
)

type AudioStyleFormat string

const (
	AudioDeepDive AudioStyleFormat = "deep_dive"
	AudioBrief    AudioStyleFormat = "brief"
	AudioCritique AudioStyleFormat = "critique"
	AudioDebate   AudioStyleFormat = "debate"
)

type AudioLength string

const (
	AudioShort   AudioLength = "short"
	AudioDefault AudioLength = "default"
	AudioLong    AudioLength = "long"
)

type VideoFormat string

const (
	VideoExplainer VideoFormat = "explainer"
	VideoBrief     VideoFormat = "brief"
	VideoCinematic VideoFormat = "cinematic"
)

type VideoStyle string

const (
	VideoStyleAuto       VideoStyle = "auto"
	VideoStyleClassic    VideoStyle = "classic"
	VideoStyleWhiteboard VideoStyle = "whiteboard"
	VideoStyleKawaii     VideoStyle = "kawaii"
	VideoStyleAnime      VideoStyle = "anime"
	VideoStyleWatercolor VideoStyle = "watercolor"
	VideoStyleRetroPrint VideoStyle = "retro_print"
)

type ReportTemplate string

const (
	ReportBriefingDoc ReportTemplate = "briefing_doc"
	ReportStudyGuide  ReportTemplate = "study_guide"
	ReportBlogPost    ReportTemplate = "blog_post"
	ReportCustom      ReportTemplate = "custom"
)

type QuizQuantity string

const (
	QuizFewer    QuizQuantity = "fewer"
	QuizStandard QuizQuantity = "standard"
)

type QuizDifficulty string

const (
	QuizEasy   QuizDifficulty = "easy"
	QuizMedium QuizDifficulty = "medium"
	QuizHard   QuizDifficulty = "hard"
)

type InfographicOrientation string

const (
	InfographicLandscape InfographicOrientation = "landscape"
	InfographicPortrait  InfographicOrientation = "portrait"
	InfographicSquare    InfographicOrientation = "square"
)

type InfographicDetail string

const (
	InfographicConcise  InfographicDetail = "concise"
	InfographicStandard InfographicDetail = "standard"
	InfographicDetailed InfographicDetail = "detailed"
)

type InfographicStyle string

const (
	InfographicSketchNote   InfographicStyle = "sketch_note"
	InfographicProfessional InfographicStyle = "professional"
	InfographicBentoGrid    InfographicStyle = "bento_grid"
)

type SlideDeckFormat string

const (
	SlideDetailed  SlideDeckFormat = "detailed"
	SlidePresenter SlideDeckFormat = "presenter"
)

type SlideDeckLength string

const (
	SlideDefault SlideDeckLength = "default"
	SlideShort   SlideDeckLength = "short"
)

type WorkflowStatus string

const (
	StatusPending          WorkflowStatus = "pending"
	StatusCreatingNotebook WorkflowStatus = "creating_notebook"
	StatusAddingSource     WorkflowStatus = "adding_source"
	StatusNavigatingStudio WorkflowStatus = "navigating_studio"
	StatusConfiguring      WorkflowStatus = "configuring"
	StatusGenerating       WorkflowStatus = "generating"
	StatusDownloading      WorkflowStatus = "downloading"
	StatusCompleted        WorkflowStatus = "completed"
	StatusFailed           WorkflowStatus = "failed"
)

// SessionCookie represents a domain-scoped cookie for cross-domain requests.
type SessionCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	HttpOnly bool   `json:"httpOnly,omitempty"`
}

// NotebookRpcSession holds all tokens and cookies for API calls.
type NotebookRpcSession struct {
	AT        string          `json:"at"`
	BL        string          `json:"bl"`
	FSID      string          `json:"fsid"`
	Cookies   string          `json:"cookies"`
	CookieJar []SessionCookie `json:"cookieJar,omitempty"`
	UserAgent string          `json:"userAgent"`
	Language  string          `json:"language,omitempty"`
}

type SourceInput struct {
	Type         SourceType
	URL          string
	Text         string
	Topic        string
	FilePath     string
	ResearchMode ResearchMode
}

type NotebookInfo struct {
	ID          string
	Title       string
	SourceCount *int
	CreatedAt   *[2]int
	UpdatedAt   *[2]int
}

type SourceInfo struct {
	ID         string
	Title      string
	WordCount  *int
	StatusCode *int
	URL        string
	CreatedAt  *[2]int
}

type ArtifactInfo struct {
	ID              string
	Title           string
	Type            int
	DownloadURL     string
	StreamURL       string
	HlsURL          string
	DashURL         string
	DurationSeconds *int
	DurationNanos   *int
	SourceIDs       []string
}

type ResearchResult struct {
	URL         string
	Title       string
	Description string
}

type StudioAudioType struct {
	ID          int
	Name        string
	Description string
}

type StudioDocType struct {
	Name        string
	Description string
}

type StudioConfig struct {
	AudioTypes     []StudioAudioType
	ExplainerTypes []StudioAudioType
	SlideTypes     []StudioAudioType
	DocTypes       []StudioDocType
}

type AccountInfo struct {
	PlanType        int
	NotebookLimit   int
	SourceLimit     int
	SourceWordLimit int
	IsPlus          bool
}

type WorkflowProgress struct {
	Status  WorkflowStatus
	Message string
}

type FlashcardEntry struct {
	Front string
	Back  string
}

// ArtifactOption is the interface for all artifact generation options.
type ArtifactOption interface {
	ArtifactType() string
}

type AudioArtifactOptions struct {
	Instructions string
	Language     string
	Format       AudioStyleFormat
	Length       AudioLength
}

func (o AudioArtifactOptions) ArtifactType() string { return "audio" }

type ReportArtifactOptions struct {
	Template     ReportTemplate
	Instructions string
	Language     string
}

func (o ReportArtifactOptions) ArtifactType() string { return "report" }

type VideoArtifactOptions struct {
	Instructions string
	Language     string
	Format       VideoFormat
	Style        VideoStyle
}

func (o VideoArtifactOptions) ArtifactType() string { return "video" }

type QuizArtifactOptions struct {
	Instructions string
	Language     string
	Quantity     QuizQuantity
	Difficulty   QuizDifficulty
}

func (o QuizArtifactOptions) ArtifactType() string { return "quiz" }

type FlashcardsArtifactOptions struct {
	Instructions string
	Language     string
	Quantity     QuizQuantity
	Difficulty   QuizDifficulty
}

func (o FlashcardsArtifactOptions) ArtifactType() string { return "flashcards" }

type InfographicArtifactOptions struct {
	Instructions string
	Language     string
	Orientation  InfographicOrientation
	Detail       InfographicDetail
	Style        InfographicStyle
}

func (o InfographicArtifactOptions) ArtifactType() string { return "infographic" }

type SlideDeckArtifactOptions struct {
	Instructions string
	Language     string
	Format       SlideDeckFormat
	Length       SlideDeckLength
}

func (o SlideDeckArtifactOptions) ArtifactType() string { return "slide_deck" }

type DataTableArtifactOptions struct {
	Instructions string
	Language     string
}

func (o DataTableArtifactOptions) ArtifactType() string { return "data_table" }

// Workflow options

type AudioOverviewOptions struct {
	Source       SourceInput
	Language     AudioLanguage
	Instructions string
	Format       AudioStyleFormat
	Length       AudioLength
	OutputDir    string
}

type MindMapOptions struct {
	Source    SourceInput
	OutputDir string
}

type FlashcardsOptions struct {
	Source       SourceInput
	OutputDir    string
	Instructions string
	Language     string
	Quantity     QuizQuantity
	Difficulty   QuizDifficulty
}

type ReportOptions struct {
	Source       SourceInput
	OutputDir    string
	Template     ReportTemplate
	Instructions string
	Language     string
}

type VideoOptions struct {
	Source       SourceInput
	OutputDir    string
	Format       VideoFormat
	Style        VideoStyle
	Instructions string
	Language     string
}

type QuizOptions struct {
	Source       SourceInput
	OutputDir    string
	Instructions string
	Language     string
	Quantity     QuizQuantity
	Difficulty   QuizDifficulty
}

type InfographicOptions struct {
	Source       SourceInput
	OutputDir    string
	Instructions string
	Language     string
	Orientation  InfographicOrientation
	Detail       InfographicDetail
	Style        InfographicStyle
}

type SlideDeckOptions struct {
	Source       SourceInput
	OutputDir    string
	Instructions string
	Language     string
	Format       SlideDeckFormat
	Length       SlideDeckLength
}

type DataTableOptions struct {
	Source       SourceInput
	OutputDir    string
	Instructions string
	Language     string
}

type AnalyzeOptions struct {
	Source   SourceInput
	Question string
}

type ChatOptions struct {
	Message string
}

// Workflow results

type AudioOverviewResult struct {
	AudioPath   string
	NotebookURL string
}

type MindMapResult struct {
	ImagePath   string
	NotebookURL string
}

type FlashcardsResult struct {
	HTMLPath     string
	Cards       []FlashcardEntry
	NotebookURL string
}

type ReportResult struct {
	MarkdownPath string
	NotebookURL  string
}

type VideoResult struct {
	VideoURL    string
	NotebookURL string
}

type QuizResult struct {
	HTMLPath    string
	NotebookURL string
}

type InfographicResult struct {
	ImagePath   string
	NotebookURL string
}

type SlideDeckResult struct {
	PptxPath    string
	PdfPath     string
	NotebookURL string
}

type DataTableResult struct {
	CsvPath     string
	NotebookURL string
}

type AnalyzeResult struct {
	Answer      string
	NotebookURL string
}

type ChatResult struct {
	Response string
}
