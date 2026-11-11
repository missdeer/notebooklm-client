package client

import (
	"context"
	"fmt"

	"github.com/missdeer/notebooklm-client/internal/download"
)

// DownloadArtifact downloads an existing artifact belonging to notebookID.
// It never generates content or substitutes another artifact when the ID is missing.
func (c *NotebookClient) DownloadArtifact(ctx context.Context, notebookID, artifactID, outputDir, format string) (string, error) {
	artifacts, err := c.GetArtifacts(ctx, notebookID)
	if err != nil {
		return "", err
	}
	for _, artifact := range artifacts {
		if artifact.ID == artifactID {
			return download.SaveArtifact(ctx, c.rpcCaller(), c.GetInteractiveHTML, c.MakeDownloadFn(), artifact, outputDir, format)
		}
	}
	return "", fmt.Errorf("artifact %s not found in notebook %s", artifactID, notebookID)
}
