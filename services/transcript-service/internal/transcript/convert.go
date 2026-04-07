package transcript

import (
	"fmt"
	"strings"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
)

type ConvertToReadableParams struct {
	Segments []model.RawSegment
}

func ConvertToReadable(params ConvertToReadableParams) []model.ReadableBlock {
	blocks := make([]model.ReadableBlock, 0, len(params.Segments))

	for _, segment := range params.Segments {
		if len(segment.Words) == 0 {
			continue
		}

		words := make([]string, 0, len(segment.Words))
		for _, w := range segment.Words {
			words = append(words, w.Text)
		}

		startSeconds := segment.Words[0].StartTime
		timestamp := formatTimestamp(startSeconds)

		text := strings.Join(words, " ")
		text = cleanTranscriptText(text)

		speaker := segment.Speaker
		if speaker == "" {
			speaker = "Unknown Speaker"
		}

		blocks = append(blocks, model.ReadableBlock{
			Speaker:   speaker,
			Timestamp: timestamp,
			Text:      text,
		})
	}

	return mergeConsecutiveSpeakerBlocks(blocks)
}

func mergeConsecutiveSpeakerBlocks(blocks []model.ReadableBlock) []model.ReadableBlock {
	if len(blocks) <= 1 {
		return blocks
	}

	merged := make([]model.ReadableBlock, 0, len(blocks))
	current := blocks[0]

	for i := 1; i < len(blocks); i++ {
		if blocks[i].Speaker == current.Speaker {
			current.Text = current.Text + " " + blocks[i].Text
		} else {
			merged = append(merged, current)
			current = blocks[i]
		}
	}
	merged = append(merged, current)

	return merged
}

func formatTimestamp(totalSeconds float64) string {
	minutes := int(totalSeconds) / 60
	seconds := int(totalSeconds) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

func cleanTranscriptText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "  ", " ")
	return text
}
