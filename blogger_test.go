package main

import (
	"context"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// Get test access token using GooglePlayGround
	// Scopes:
	// https://www.googleapis.com/auth/blogger
	// https://picasaweb.google.com/data/
	GoogleAPIAccessToken = "YOUR_ACCESS_TOKEN"

	TestUploadFilePath = ""
	TestUploadFileMime = "image/jpeg"
	TestUploadBlogId   = ""
)

func testClient() *http.Client {
	config := &oauth2.Config{
		Endpoint: google.Endpoint,
		Scopes: []string{
			"https://www.googleapis.com/auth/blogger",
			"https://picasaweb.google.com/data/",
		},
	}

	token := oauth2.Token{
		AccessToken: GoogleAPIAccessToken,
		TokenType:   "Bearer",
	}

	return config.Client(context.Background(), &token)
}

func TestBloggerList(t *testing.T) {
	c := testClient()
	l, err := BlogList(c)
	if err != nil {
		t.Fatal(err)
	}

	for _, i := range l {
		t.Logf("%s > %s > %s", i.Id, i.Name, i.Description)
	}
}

func TestBloggerPost(t *testing.T) {
	c := testClient()

	b, err := ioutil.ReadFile(TestUploadFilePath)
	if err != nil {
		t.Fatal(err)
	}

	testTitle := "This is golang test"
	testLabels := []string{"Golang", "Autotest"}
	testContent := `<p>測試文字</p><p>第二段</p><img src="IMAGE-UPLOAD-URL"/>`

	bloggerRes := []BloggerResource{
		BloggerResource{
			FileName: filepath.Base(TestUploadFilePath),
			Mime:     TestUploadFileMime,
			Summary:  "Test upload",
			Raws:     b,
		},
	}

	bloggerRes, err = UploadBloggerResources(c, TestUploadBlogId, bloggerRes)
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range bloggerRes {
		testContent = strings.Replace(testContent, "IMAGE-UPLOAD-URL", r.URL, -1)
	}

	post, err := PostBloggerPost(c, TestUploadBlogId, testTitle, testContent, testLabels, true)
	if err != nil {
		t.Fatal(err)
	} else {
		t.Logf(post.Id)
	}
}
