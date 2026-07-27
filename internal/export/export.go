// Package export provides session export functionality for generating
// HTML and Markdown reports from Argus sessions.
package export

import (
	"fmt"
	"os"
	"path/filepath"

	"argus-desktop/internal/session"
)

// ExportFormat represents the output format for session export.
type ExportFormat string

const (
	// FormatHTML exports the session as an HTML report.
	FormatHTML ExportFormat = "html"
	// FormatMarkdown exports the session as a Markdown document.
	FormatMarkdown ExportFormat = "markdown"
)

// ExportOptions configures the export behavior.
type ExportOptions struct {
	// Format is the output format (html or markdown).
	Format ExportFormat
	// OutputDir is the directory where the report will be saved.
	// If empty, uses the default reports directory.
	OutputDir string
	// SessionID is the ID of the session to export.
	SessionID string
}

// ExportResult contains the result of a successful export.
type ExportResult struct {
	// FilePath is the full path to the exported file.
	FilePath string
	// Format is the format of the exported file.
	Format ExportFormat
	// FileSize is the size of the exported file in bytes.
	FileSize int64
}

// ExportSession generates a report for the given session.
// It returns the file path of the exported report.
func ExportSession(sess *session.Session, diff string, opts ExportOptions) (*ExportResult, error) {
	if sess == nil {
		return nil, fmt.Errorf("export: session cannot be nil")
	}

	if opts.Format == "" {
		opts.Format = FormatHTML
	}

	// Determine output directory
	outputDir := opts.OutputDir
	if outputDir == "" {
		var err error
		outputDir, err = getDefaultExportDir()
		if err != nil {
			return nil, fmt.Errorf("export: failed to get default export directory: %w", err)
		}
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("export: failed to create output directory: %w", err)
	}

	// Generate filename
	filename := generateFilename(sess, opts.Format)
	outputPath := filepath.Join(outputDir, filename)

	// Generate report based on format
	var err error
	switch opts.Format {
	case FormatHTML:
		err = generateHTML(sess, diff, outputPath)
	case FormatMarkdown:
		err = generateMarkdown(sess, diff, outputPath)
	default:
		return nil, fmt.Errorf("export: unsupported format: %s", opts.Format)
	}

	if err != nil {
		return nil, fmt.Errorf("export: failed to generate report: %w", err)
	}

	// Get file info
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("export: failed to stat output file: %w", err)
	}

	return &ExportResult{
		FilePath: outputPath,
		Format:   opts.Format,
		FileSize: info.Size(),
	}, nil
}

// generateFilename creates a filename for the exported report.
func generateFilename(sess *session.Session, format ExportFormat) string {
	timestamp := sess.StartedAt.Format("20060102-150405")
	sessionID := sess.ID
	if len(sessionID) > 8 {
		sessionID = sessionID[:8]
	}

	var ext string
	switch format {
	case FormatHTML:
		ext = "html"
	case FormatMarkdown:
		ext = "md"
	default:
		ext = "txt"
	}

	return fmt.Sprintf("argus-report-%s-%s.%s", timestamp, sessionID, ext)
}

// getDefaultExportDir returns the default directory for exported reports.
func getDefaultExportDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, "Argus", "reports"), nil
}
