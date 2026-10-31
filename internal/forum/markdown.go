package forum

import (
	"bytes"
	"regexp"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

// Sanitize after parsing. Raw HTML is disabled in goldmark as well; stored bodies
// remain the original source and can always be edited or switched back to text.
var markdownPolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("class").Matching(regexp.MustCompile("^language-[a-zA-Z0-9_+.-]{1,64}$")).OnElements("code")
	p.AllowAttrs("type").Matching(regexp.MustCompile("^checkbox$")).OnElements("input")
	p.AllowAttrs("checked", "disabled").OnElements("input")
	p.AddTargetBlankToFullyQualifiedLinks(true)
	p.RequireNoReferrerOnLinks(true)
	return p
}()

func renderMarkdown(body, scope string) (string, error) {
	md := goldmark.New(goldmark.WithParserOptions(parser.WithAutoHeadingID()), goldmark.WithExtensions(
		extension.GFM,
		extension.DefinitionList,
		extension.NewFootnote(extension.WithFootnoteIDPrefix(scope+"-")),
	))
	var output bytes.Buffer
	if err := md.Convert([]byte(body), &output); err != nil {
		return "", err
	}
	return markdownPolicy.Sanitize(output.String()), nil
}

func renderPostMarkdown(post *Post) error {
	post.BodyHTML = ""
	if post.Markdown {
		rendered, err := renderMarkdown(post.Body, "md-post-"+post.ID)
		if err != nil {
			return err
		}
		post.BodyHTML = rendered
	}
	for i := range post.Replies {
		if err := renderReplyMarkdown(&post.Replies[i]); err != nil {
			return err
		}
	}
	return nil
}

func renderReplyMarkdown(reply *Reply) error {
	reply.BodyHTML = ""
	if reply.Markdown {
		rendered, err := renderMarkdown(reply.Body, "md-reply-"+reply.ID)
		if err != nil {
			return err
		}
		reply.BodyHTML = rendered
	}
	return nil
}

func markdownChoice(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

// Omitted format fields in older clients must preserve an existing choice.
func markdownArgument(value *bool) []bool {
	if value == nil {
		return nil
	}
	return []bool{*value}
}
