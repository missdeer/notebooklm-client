package client

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/missdeer/notebooklm-client/internal/download"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/types"
	"github.com/missdeer/notebooklm-client/internal/util"
)

type ProgressFn func(types.WorkflowProgress)

func progress(fn ProgressFn, status types.WorkflowStatus, msg string) {
	if fn != nil {
		fn(types.WorkflowProgress{Status: status, Message: msg})
	}
}

func addSourceFromInput(ctx context.Context, c *NotebookClient, notebookID string, source types.SourceInput) ([]string, error) {
	switch source.Type {
	case types.SourceURL:
		id, _, err := c.AddURLSource(ctx, notebookID, source.URL)
		if err != nil {
			return nil, err
		}
		return []string{id}, nil
	case types.SourceText:
		id, _, err := c.AddTextSource(ctx, notebookID, "Pasted Text", source.Text)
		if err != nil {
			return nil, err
		}
		return []string{id}, nil
	case types.SourceFile:
		id, _, err := c.AddFileSource(ctx, notebookID, source.FilePath)
		if err != nil {
			return nil, err
		}
		return []string{id}, nil
	case types.SourceResearch:
		mode := source.ResearchMode
		if mode == "" {
			mode = types.ResearchFast
		}
		_, _, err := c.AddTextSource(ctx, notebookID, "Research Topic", source.Topic)
		if err != nil {
			return nil, err
		}
		_, _, err = c.CreateWebSearch(ctx, notebookID, source.Topic, mode)
		if err != nil {
			return nil, err
		}
		results, report, err := c.PollResearchResults(ctx, notebookID, 0)
		if err != nil {
			return nil, err
		}
		if report != "" {
			c.AddTextSource(ctx, notebookID, "Research Report: "+source.Topic, report)
		}
		for _, r := range results {
			c.AddURLSource(ctx, notebookID, r.URL)
		}
		pollSourcesReady(ctx, c, notebookID, 120*time.Second)
		_, sources, err := c.GetNotebookDetail(ctx, notebookID)
		if err != nil {
			return nil, err
		}
		ids := make([]string, len(sources))
		for i, s := range sources {
			ids[i] = s.ID
		}
		return ids, nil
	default:
		return nil, fmt.Errorf("unknown source type: %s", source.Type)
	}
}

func pollSourcesReady(ctx context.Context, c *NotebookClient, notebookID string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	pollCount := 0
	for time.Now().Before(deadline) {
		_, sources, err := c.GetNotebookDetail(ctx, notebookID)
		if err == nil && len(sources) > 0 {
			allReady := true
			for _, s := range sources {
				if s.WordCount == nil || *s.WordCount == 0 {
					allReady = false
					break
				}
			}
			if allReady {
				return
			}
		}
		pollCount++
		delay := int(math.Min(float64(3000+pollCount*1500), 15000))
		util.HumanSleep(ctx, delay)
	}
	log.Println("NotebookLM: Source processing may not have completed within timeout")
}

func pollArtifactReady(ctx context.Context, c *NotebookClient, notebookID, artifactID string, timeout time.Duration) (*types.ArtifactInfo, error) {
	deadline := time.Now().Add(timeout)
	pollCount := 0
	for time.Now().Before(deadline) {
		artifacts, err := c.GetArtifacts(ctx, notebookID)
		if err != nil {
			return nil, err
		}
		for i := range artifacts {
			a := &artifacts[i]
			if a.ID != artifactID {
				continue
			}
			isMedia := a.Type == rpc.ArtifactAudio || a.Type == rpc.ArtifactVideo
			if isMedia {
				if a.DownloadURL != "" || a.StreamURL != "" || a.HlsURL != "" {
					return a, nil
				}
			} else {
				return a, nil
			}
		}
		pollCount++
		delay := int(math.Min(float64(5000+pollCount*2500), 30000))
		util.HumanSleep(ctx, delay)
	}
	return nil, fmt.Errorf("artifact generation timed out")
}

func (c *NotebookClient) RunAudioOverview(ctx context.Context, opts types.AudioOverviewOptions, onProgress ProgressFn) (*types.AudioOverviewResult, error) {
	if err := c.EnsureConnected(); err != nil {
		return nil, err
	}

	progress(onProgress, types.StatusCreatingNotebook, "Creating notebook...")
	notebookID, err := c.CreateNotebook(ctx)
	if err != nil {
		return nil, err
	}

	progress(onProgress, types.StatusAddingSource, fmt.Sprintf("Adding source (%s)...", opts.Source.Type))
	sourceIDs, err := addSourceFromInput(ctx, c, notebookID, opts.Source)
	if err != nil {
		return nil, err
	}

	progress(onProgress, types.StatusConfiguring, "Waiting for source processing...")
	pollSourcesReady(ctx, c, notebookID, 120*time.Second)

	progress(onProgress, types.StatusGenerating, "Generating audio overview...")
	artifactID, _, err := c.GenerateArtifact(ctx, notebookID, sourceIDs,
		types.AudioArtifactOptions{
			Language:     string(opts.Language),
			Instructions: opts.Instructions,
			Format:       opts.Format,
			Length:        opts.Length,
		})
	if err != nil {
		return nil, err
	}

	progress(onProgress, types.StatusGenerating, "Waiting for audio generation...")
	artifact, err := pollArtifactReady(ctx, c, notebookID, artifactID, 30*time.Minute)
	if err != nil {
		return nil, err
	}

	progress(onProgress, types.StatusDownloading, "Downloading audio...")
	audioPath, err := c.DownloadFile(ctx, artifact.DownloadURL, opts.OutputDir,
		fmt.Sprintf("audio_%d.mp3", time.Now().UnixMilli()))
	if err != nil {
		return nil, err
	}

	progress(onProgress, types.StatusCompleted, "Audio overview complete!")
	return &types.AudioOverviewResult{
		AudioPath:   audioPath,
		NotebookURL: rpc.BaseURL + "/notebook/" + notebookID,
	}, nil
}

func (c *NotebookClient) RunReport(ctx context.Context, opts types.ReportOptions, onProgress ProgressFn) (*types.ReportResult, error) {
	if err := c.EnsureConnected(); err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCreatingNotebook, "Creating notebook...")
	notebookID, err := c.CreateNotebook(ctx)
	if err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusAddingSource, fmt.Sprintf("Adding source (%s)...", opts.Source.Type))
	sourceIDs, err := addSourceFromInput(ctx, c, notebookID, opts.Source)
	if err != nil {
		return nil, err
	}
	pollSourcesReady(ctx, c, notebookID, 120*time.Second)

	progress(onProgress, types.StatusGenerating, "Generating report...")
	artifactID, _, err := c.GenerateArtifact(ctx, notebookID, sourceIDs,
		types.ReportArtifactOptions{Template: opts.Template, Instructions: opts.Instructions, Language: opts.Language})
	if err != nil {
		return nil, err
	}
	if _, err := pollArtifactReady(ctx, c, notebookID, artifactID, 5*time.Minute); err != nil {
		return nil, err
	}

	progress(onProgress, types.StatusDownloading, "Saving report...")
	mdPath, err := download.SaveReport(ctx, c.rpcCaller(), artifactID, opts.OutputDir)
	if err != nil {
		return nil, err
	}

	progress(onProgress, types.StatusCompleted, "Report complete!")
	return &types.ReportResult{
		MarkdownPath: mdPath,
		NotebookURL:  rpc.BaseURL + "/notebook/" + notebookID,
	}, nil
}

func (c *NotebookClient) RunVideo(ctx context.Context, opts types.VideoOptions, onProgress ProgressFn) (*types.VideoResult, error) {
	if err := c.EnsureConnected(); err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCreatingNotebook, "Creating notebook...")
	notebookID, err := c.CreateNotebook(ctx)
	if err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusAddingSource, fmt.Sprintf("Adding source (%s)...", opts.Source.Type))
	sourceIDs, err := addSourceFromInput(ctx, c, notebookID, opts.Source)
	if err != nil {
		return nil, err
	}
	pollSourcesReady(ctx, c, notebookID, 120*time.Second)

	progress(onProgress, types.StatusGenerating, "Generating video...")
	artifactID, _, err := c.GenerateArtifact(ctx, notebookID, sourceIDs,
		types.VideoArtifactOptions{Format: opts.Format, Style: opts.Style, Instructions: opts.Instructions, Language: opts.Language})
	if err != nil {
		return nil, err
	}
	artifact, err := pollArtifactReady(ctx, c, notebookID, artifactID, 30*time.Minute)
	if err != nil {
		return nil, err
	}
	videoURL := artifact.StreamURL
	if videoURL == "" {
		videoURL = artifact.HlsURL
	}
	if videoURL == "" {
		videoURL = artifact.DownloadURL
	}

	progress(onProgress, types.StatusCompleted, "Video complete!")
	return &types.VideoResult{
		VideoURL:    videoURL,
		NotebookURL: rpc.BaseURL + "/notebook/" + notebookID,
	}, nil
}

func (c *NotebookClient) RunQuiz(ctx context.Context, opts types.QuizOptions, onProgress ProgressFn) (*types.QuizResult, error) {
	if err := c.EnsureConnected(); err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCreatingNotebook, "Creating notebook...")
	notebookID, err := c.CreateNotebook(ctx)
	if err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusAddingSource, fmt.Sprintf("Adding source (%s)...", opts.Source.Type))
	sourceIDs, err := addSourceFromInput(ctx, c, notebookID, opts.Source)
	if err != nil {
		return nil, err
	}
	pollSourcesReady(ctx, c, notebookID, 120*time.Second)

	progress(onProgress, types.StatusGenerating, "Generating quiz...")
	artifactID, _, err := c.GenerateArtifact(ctx, notebookID, sourceIDs,
		types.QuizArtifactOptions{Instructions: opts.Instructions, Language: opts.Language, Quantity: opts.Quantity, Difficulty: opts.Difficulty})
	if err != nil {
		return nil, err
	}
	if _, err := pollArtifactReady(ctx, c, notebookID, artifactID, 5*time.Minute); err != nil {
		return nil, err
	}

	progress(onProgress, types.StatusDownloading, "Saving quiz...")
	htmlPath, err := download.SaveQuizHTML(ctx, c.GetInteractiveHTML, artifactID, opts.OutputDir, "quiz")
	if err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCompleted, "Quiz generated!")
	return &types.QuizResult{HTMLPath: htmlPath, NotebookURL: rpc.BaseURL + "/notebook/" + notebookID}, nil
}

func (c *NotebookClient) RunFlashcards(ctx context.Context, opts types.FlashcardsOptions, onProgress ProgressFn) (*types.FlashcardsResult, error) {
	if err := c.EnsureConnected(); err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCreatingNotebook, "Creating notebook...")
	notebookID, err := c.CreateNotebook(ctx)
	if err != nil {
		return nil, err
	}
	sourceIDs, err := addSourceFromInput(ctx, c, notebookID, opts.Source)
	if err != nil {
		return nil, err
	}
	pollSourcesReady(ctx, c, notebookID, 120*time.Second)

	artifactID, _, err := c.GenerateArtifact(ctx, notebookID, sourceIDs,
		types.FlashcardsArtifactOptions{Instructions: opts.Instructions, Language: opts.Language, Quantity: opts.Quantity, Difficulty: opts.Difficulty})
	if err != nil {
		return nil, err
	}
	if _, err := pollArtifactReady(ctx, c, notebookID, artifactID, 5*time.Minute); err != nil {
		return nil, err
	}

	htmlPath, err := download.SaveQuizHTML(ctx, c.GetInteractiveHTML, artifactID, opts.OutputDir, "flashcards")
	if err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCompleted, "Flashcards generated!")
	return &types.FlashcardsResult{HTMLPath: htmlPath, NotebookURL: rpc.BaseURL + "/notebook/" + notebookID}, nil
}

func (c *NotebookClient) RunInfographic(ctx context.Context, opts types.InfographicOptions, onProgress ProgressFn) (*types.InfographicResult, error) {
	if err := c.EnsureConnected(); err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCreatingNotebook, "Creating notebook...")
	notebookID, err := c.CreateNotebook(ctx)
	if err != nil {
		return nil, err
	}
	sourceIDs, err := addSourceFromInput(ctx, c, notebookID, opts.Source)
	if err != nil {
		return nil, err
	}
	pollSourcesReady(ctx, c, notebookID, 120*time.Second)

	artifactID, _, err := c.GenerateArtifact(ctx, notebookID, sourceIDs,
		types.InfographicArtifactOptions{Instructions: opts.Instructions, Language: opts.Language, Orientation: opts.Orientation, Detail: opts.Detail, Style: opts.Style})
	if err != nil {
		return nil, err
	}
	if _, err := pollArtifactReady(ctx, c, notebookID, artifactID, 5*time.Minute); err != nil {
		return nil, err
	}

	imagePath, err := download.SaveInfographic(ctx, c.rpcCaller(), c.MakeDownloadFn(), artifactID, opts.OutputDir)
	if err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCompleted, "Infographic complete!")
	return &types.InfographicResult{ImagePath: imagePath, NotebookURL: rpc.BaseURL + "/notebook/" + notebookID}, nil
}

func (c *NotebookClient) RunSlideDeck(ctx context.Context, opts types.SlideDeckOptions, onProgress ProgressFn) (*types.SlideDeckResult, error) {
	if err := c.EnsureConnected(); err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCreatingNotebook, "Creating notebook...")
	notebookID, err := c.CreateNotebook(ctx)
	if err != nil {
		return nil, err
	}
	sourceIDs, err := addSourceFromInput(ctx, c, notebookID, opts.Source)
	if err != nil {
		return nil, err
	}
	pollSourcesReady(ctx, c, notebookID, 120*time.Second)

	artifactID, _, err := c.GenerateArtifact(ctx, notebookID, sourceIDs,
		types.SlideDeckArtifactOptions{Instructions: opts.Instructions, Language: opts.Language, Format: opts.Format, Length: opts.Length})
	if err != nil {
		return nil, err
	}
	if _, err := pollArtifactReady(ctx, c, notebookID, artifactID, 5*time.Minute); err != nil {
		return nil, err
	}

	pptx, pdf, err := download.SaveSlideDeck(ctx, c.rpcCaller(), c.MakeDownloadFn(), artifactID, opts.OutputDir)
	if err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCompleted, "Slide deck complete!")
	return &types.SlideDeckResult{PptxPath: pptx, PdfPath: pdf, NotebookURL: rpc.BaseURL + "/notebook/" + notebookID}, nil
}

func (c *NotebookClient) RunDataTable(ctx context.Context, opts types.DataTableOptions, onProgress ProgressFn) (*types.DataTableResult, error) {
	if err := c.EnsureConnected(); err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCreatingNotebook, "Creating notebook...")
	notebookID, err := c.CreateNotebook(ctx)
	if err != nil {
		return nil, err
	}
	sourceIDs, err := addSourceFromInput(ctx, c, notebookID, opts.Source)
	if err != nil {
		return nil, err
	}
	pollSourcesReady(ctx, c, notebookID, 120*time.Second)

	artifactID, _, err := c.GenerateArtifact(ctx, notebookID, sourceIDs,
		types.DataTableArtifactOptions{Instructions: opts.Instructions, Language: opts.Language})
	if err != nil {
		return nil, err
	}
	if _, err := pollArtifactReady(ctx, c, notebookID, artifactID, 5*time.Minute); err != nil {
		return nil, err
	}

	csvPath, err := download.SaveDataTable(ctx, c.rpcCaller(), artifactID, opts.OutputDir)
	if err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCompleted, "Data table complete!")
	return &types.DataTableResult{CsvPath: csvPath, NotebookURL: rpc.BaseURL + "/notebook/" + notebookID}, nil
}

func (c *NotebookClient) RunAnalyze(ctx context.Context, opts types.AnalyzeOptions, onProgress ProgressFn) (*types.AnalyzeResult, error) {
	if err := c.EnsureConnected(); err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCreatingNotebook, "Creating notebook...")
	notebookID, err := c.CreateNotebook(ctx)
	if err != nil {
		return nil, err
	}
	sourceIDs, err := addSourceFromInput(ctx, c, notebookID, opts.Source)
	if err != nil {
		return nil, err
	}
	pollSourcesReady(ctx, c, notebookID, 120*time.Second)

	progress(onProgress, types.StatusGenerating, "Analyzing...")
	text, _, err := c.SendChat(ctx, notebookID, opts.Question, sourceIDs)
	if err != nil {
		return nil, err
	}
	progress(onProgress, types.StatusCompleted, "Analysis complete!")
	return &types.AnalyzeResult{Answer: text, NotebookURL: rpc.BaseURL + "/notebook/" + notebookID}, nil
}

// Ensure outputDir exists.
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
