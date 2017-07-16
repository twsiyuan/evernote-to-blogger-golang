package main

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"strings"

	"github.com/twsiyuan/evernote-sdk-golang/edamutil"
	"github.com/yosssi/gohtml"
)

const (
	EvernoteEnvironment = edamutil.PRODUCTION
	EvernoteAccessToken = "YOUR_EVERNOTE_ACCESS_TOKEN"
	EvernoteNoteGUID    = "YOUR_NOTE_GUID"
	OutputFolderPath    = ""

	OutputTemplate = `<!DOCTYPE HTML><html><head>
	   <meta charset="utf-8">
	     <title>{evernote-note-title}</title>
	     <meta name="evernote-note-guid" content="{evernote-note-guid}">
	     <meta name="evernote-note-tags" content="{evernote-note-tags}">
	   </head>
	   <body>{evernote-note-body}</body>
	   </html>`
)

func TestNoteToFile(t *testing.T) {
	us, err := edamutil.NewUserStore(EvernoteEnvironment)
	if err != nil {
		t.Fatal(err)
	}

	ns, err := edamutil.NewNoteStore(us, EvernoteAccessToken)
	if err != nil {
		t.Fatal(err)
	}

	// Get note from Evernote
	note, err := NoteMarkdown2Html(ns, EvernoteAccessToken, EvernoteNoteGUID)
	if err != nil {
		t.Fatal(err)
	}

	// Write Files
	outputPath := path.Join(OutputFolderPath, EvernoteNoteGUID)
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		if err := os.MkdirAll(outputPath, os.ModePerm); err != nil {
			t.Fatal(err)
		}
	}

	for _, resource := range note.Resources {
		filePath := path.Join(outputPath, resource.Path)
		if err := ioutil.WriteFile(filePath, resource.Raws, 0644); err != nil {
			t.Errorf("Resource[%s] write error, %v", resource.Path, err)
			continue
		}
		note.Content = strings.Replace(note.Content, `src="`+resource.Hash+`"`, `src="./`+EvernoteNoteGUID+"/"+resource.Path+`"`, -1)
	}

	outputHtmlPath := path.Join(OutputFolderPath, EvernoteNoteGUID+".html")
	html := strings.Replace(OutputTemplate, "{evernote-note-title}", note.Title, -1)
	html = strings.Replace(html, "{evernote-note-guid}", note.GUID, -1)
	html = strings.Replace(html, "{evernote-note-tags}", strings.Join(note.Tags, ","), -1)
	html = strings.Replace(html, "{evernote-note-body}", note.Content, -1)
	html = gohtml.Format(html)

	if err := ioutil.WriteFile(outputHtmlPath, ([]byte)(html), 0644); err != nil {
		t.Fatal(err)
	}

	t.Logf("Done, %s", outputHtmlPath)
}
