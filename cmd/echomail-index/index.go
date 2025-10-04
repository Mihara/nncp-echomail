package main

import (
	"bytes"
	"echomail/envelope"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
)

const rootIndex = `
# Echomail Message Groups

{{range $key, $value := .}}
=> {{$key}}/index.gmi 📰 {{$value}}
{{end}}
`

const groupIndex = `
# Echomail: {{ .Group }}

{{- define "link" }}
=> {{ .URL }}/index.gmi ✉️{{- .Prefix }} {{ .Text }}
{{- end -}}
{{- define "object" }}
  {{- $prefix := .Prefix | default "" -}}
  {{- $text := printf "%s: %s" .From .Subj -}}
  {{- template "link" (dict "URL" .MsgID "Prefix" $prefix "Text" $text) -}}
  {{- if .Children }}
    {{- $newPrefix := cat (repeat (len $prefix) " ") "⤷" -}}
    {{- range .Children -}}
      {{- template "object" (dict "MsgID" .MsgID "From" .From "Subj" .Subj "Children" .Children "Prefix" $newPrefix) -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- range .Messages }}
  {{- template "object" (dict "MsgID" .MsgID "From" .From "Subj" .Subj "Children" .Children "Prefix" "") -}}
{{ end }}

=> ../index.gmi 🔙 Back to groups
`

type Message struct {
	MsgID    string
	Parent   *string
	Date     *time.Time
	From     string
	Subj     string
	Children []*Message
}

func sortMessagesByDate(messages []*Message) {
	slices.SortFunc(messages, func(a, b *Message) int {
		switch {
		case a.Date == nil && b.Date == nil:
			return 0
		case a.Date == nil:
			return -1 // nil is earlier
		case b.Date == nil:
			return 1
		default:
			return a.Date.Compare(*b.Date)
		}
	})
}

func sortTreeByDate(root []*Message) {

	sortMessagesByDate(root)

	for _, child := range root {
		sortTreeByDate(child.Children)
	}
}

// This is the minimal proof of concept to make the mailer useful as is
// without an actual reader application.
func indexMail(root string) error {

	// First, go through dirs. Yes, we're assuming dirs.
	groupDirs, _ := filepath.Glob(filepath.Join(root, "*.group"))

	groups := make(map[string]string, len(groupDirs))

	// Find a message in every group to get the actual group name out of it.
	for _, dir := range groupDirs {
		msgGlob, err := filepath.Glob(filepath.Join(dir, "*", "index.gmi"))
		if err != nil {
			return err
		}
		if len(msgGlob) > 0 {
			msg, err := os.ReadFile(msgGlob[0])
			if err != nil {
				return err
			}
			fields, err := envelope.MessageHeader(msg)
			if err != nil {
				return err
			}

			groups[filepath.Base(dir)] = fields["Group"]
		}
	}

	// Save the index file.

	t := template.Must(template.New("rootIndex").Parse(rootIndex))
	var buf bytes.Buffer
	err := t.Execute(&buf, groups)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(root, "index.gmi"), buf.Bytes(), 0666)
	if err != nil {
		return err
	}

	// Now the more fun part: Going through the groups to index messages in them.

	gt := template.Must(template.New("groupIndex").Funcs(sprig.FuncMap()).Parse(groupIndex))

	for groupDir, groupName := range groups {
		groupPath := filepath.Join(root, groupDir)
		msgGlob, _ := filepath.Glob(filepath.Join(groupPath, "*", "index.gmi"))

		msgMap := make(map[string]*Message)

		for _, msg := range msgGlob {
			msgId := filepath.Base(filepath.Dir(msg))

			msgBody, err := os.ReadFile(msg)
			if err != nil {
				return err
			}
			hdr, err := envelope.MessageHeader(msgBody)
			if err != nil {
				return err
			}

			replyTo, ok := hdr["ReplyTo"]
			var parent *string
			if ok {
				parent = &replyTo
			}

			var date *time.Time
			if dateString, ok := hdr["Date"]; ok {
				thatDate, err := time.ParseInLocation(envelope.DateFormat, dateString, time.UTC)
				if err == nil {
					date = &thatDate
				}
			}

			from := hdr["From"]
			if from == "" {
				from = hdr["Sender"]
			}
			if strings.Contains(from, "[") {
				from = strings.TrimSpace(strings.Split(from, "[")[0])
			}
			subj := hdr["Subj"]

			msgMap[msgId] = &Message{
				MsgID:  msgId,
				Parent: parent,
				Date:   date,
				From:   from,
				Subj:   subj,
			}
		}

		// Now that we've made a msgMap, make a tree...
		var messages []*Message
		for _, msg := range msgMap {
			if msg.Parent != nil {
				parent, exists := msgMap[*msg.Parent]
				if exists {
					parent.Children = append(parent.Children, msg)
				} else {
					messages = append(messages, msg)
				}
			} else {
				messages = append(messages, msg)
			}
		}

		sortTreeByDate(messages)

		var buf bytes.Buffer
		err := gt.Execute(&buf, map[string]any{
			"Group":    groupName,
			"Messages": messages,
		})
		if err != nil {
			return err
		}
		err = os.WriteFile(filepath.Join(root, groupDir, "index.gmi"), buf.Bytes(), 0666)
		if err != nil {
			return err
		}

	}

	return nil
}
