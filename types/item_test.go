package hyperfeed

import (
	"testing"

	"github.com/herdworks/hyperfeed-go/types/href"
)

func TestItemDeduplicateAttachmentsMergesDuplicateMetadata(t *testing.T) {
	primaryHref, err := href.Parse("https://example.com/photo.png?utm_source=newsletter")
	if err != nil {
		t.Fatalf("href.Parse() primary error = %v", err)
	}
	duplicateHref, err := href.Parse("https://www.example.com/photo.png")
	if err != nil {
		t.Fatalf("href.Parse() duplicate error = %v", err)
	}
	posterHref, err := href.Parse("https://example.com/poster.png")
	if err != nil {
		t.Fatalf("href.Parse() poster error = %v", err)
	}
	sourceHref, err := href.Parse("https://example.com/photo-large.webp")
	if err != nil {
		t.Fatalf("href.Parse() source error = %v", err)
	}
	extraSourceHref, err := href.Parse("https://example.com/photo-large.png")
	if err != nil {
		t.Fatalf("href.Parse() extra source error = %v", err)
	}
	item := &Item{
		Attachments: []Attachment{
			{
				Href:     primaryHref,
				Mimetype: "application/octet-stream",
				Rank:     3,
				Sources: []Source{
					{Href: sourceHref, Mimetype: "application/octet-stream", Width: 1024, Height: 768},
				},
			},
			{
				Href:             duplicateHref,
				Poster:           posterHref,
				Mimetype:         "image/png",
				Title:            "Photo",
				Description:      "A duplicate attachment with richer metadata",
				Enclosure:        true,
				Download:         true,
				Size:             5,
				Duration:         6,
				PlaybackPosition: 2,
				Width:            640,
				Height:           480,
				Rank:             4,
				Sources: []Source{
					{Href: sourceHref, Mimetype: "image/webp", Width: 1024, Height: 768},
					{Href: extraSourceHref, Mimetype: "image/png", Width: 640, Height: 480},
				},
			},
		},
	}

	item.DeduplicateAttachments()

	if len(item.Attachments) != 1 {
		t.Fatalf("attachments length = %d, want 1", len(item.Attachments))
	}
	attachment := item.Attachments[0]
	if attachment.Rank != 3 {
		t.Fatalf("rank = %d, want first rank 3", attachment.Rank)
	}
	if !attachment.Enclosure || !attachment.Download {
		t.Fatalf("attachment flags = enclosure:%t download:%t, want both true", attachment.Enclosure, attachment.Download)
	}
	if attachment.Mimetype != "image/png" {
		t.Fatalf("mimetype = %q, want image/png", attachment.Mimetype)
	}
	if attachment.Title != "Photo" || attachment.Description == "" {
		t.Fatalf("metadata = (%q, %q), want merged title and description", attachment.Title, attachment.Description)
	}
	if attachment.Poster == nil || attachment.Poster.String() != "https://example.com/poster.png" {
		t.Fatalf("poster = %v, want https://example.com/poster.png", attachment.Poster)
	}
	if attachment.Size != 5 || attachment.Duration != 6 || attachment.PlaybackPosition != 2 {
		t.Fatalf("numeric metadata = size:%d duration:%d position:%d, want 5/6/2", attachment.Size, attachment.Duration, attachment.PlaybackPosition)
	}
	if attachment.Width != 640 || attachment.Height != 480 {
		t.Fatalf("dimensions = %dx%d, want 640x480", attachment.Width, attachment.Height)
	}
	if len(attachment.Sources) != 2 {
		t.Fatalf("sources length = %d, want 2", len(attachment.Sources))
	}
	if attachment.Sources[0].Mimetype != "image/webp" {
		t.Fatalf("first source mimetype = %q, want image/webp", attachment.Sources[0].Mimetype)
	}
}
