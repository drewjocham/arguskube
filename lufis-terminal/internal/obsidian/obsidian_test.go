package obsidian

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractTags(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"no tags here", nil},
		{"a #tag here", []string{"tag"}},
		{"#tag1 and #tag2", []string{"tag1", "tag2"}},
		{"code #tag; here", []string{"tag"}},
		{"#alone", []string{"alone"}},
	}
	for _, tt := range tests {
		got := extractTags(tt.line)
		assert.Equal(t, tt.want, got)
	}
}

func TestExtractWikilinks(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"no links", nil},
		{"see [[note]] here", []string{"note"}},
		{"[[note1]] and [[note2]]", []string{"note1", "note2"}},
		{"[[note|display]]", []string{"note"}},
		{"[[page#section]]", []string{"page#section"}},
	}
	for _, tt := range tests {
		got := extractWikilinks(tt.line)
		assert.Equal(t, tt.want, got)
	}
}

func TestParseNote(t *testing.T) {
	content := `---
title: Test Note
tags: dev, go
---
# Hello

This is a [[related-note]] with a #tag inside.

Another [[page]] reference.`
	note := parseNote("/tmp/test.md", content)
	assert.Equal(t, "/tmp/test.md", note.Path)
	assert.Equal(t, "test.md", note.Title)
	assert.Equal(t, "dev, go", note.Frontmatter["tags"])
	assert.Contains(t, note.Tags, "tag")
	assert.Contains(t, note.Wikilinks, "related-note")
	assert.Contains(t, note.Wikilinks, "page")
}

func TestParseNoteNoFrontmatter(t *testing.T) {
	content := "# Just a heading\n\nSome content"
	note := parseNote("/tmp/simple.md", content)
	assert.Equal(t, "simple.md", note.Title)
	assert.Empty(t, note.Frontmatter)
}
