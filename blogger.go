package main

import (
	"errors"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/tgulacsi/picago"
	"google.golang.org/api/blogger/v3"
)

var (
	BlogspotHosts = [...]string{
		"1.bp.blogspot.com",
		"2.bp.blogspot.com",
		"3.bp.blogspot.com",
		"4.bp.blogspot.com",
	}
)

func BlogList(client *http.Client) ([]*blogger.Blog, error) {
	service, err := blogger.New(client)
	if err != nil {
		return nil, err
	}

	user, err := service.Users.Get("self").Do()
	if err != nil {
		return nil, err
	}

	list, err := service.Blogs.ListByUser(user.Id).Do()
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

type BloggerResource struct {
	FileName string // Prefer filename
	Summary  string // Summary
	Mime     string // Resource mime (content-type), cuuurent support image/gif, image/jpeg, image/png
	Raws     []byte // Resource raws

	URL string // Url after uploading
}

func FindAlbumId(client *http.Client, blogName string) (string, error) {
	albums, err := picago.GetAlbums(client, "")
	if err != nil {
		return "", err
	}
	for _, album := range albums {
		if album.AlbumType == "Blogger" && album.Title == blogName {
			return album.ID, nil
		}
	}

	return "", errors.New("Can't not find Blogger album")
}

func UploadBloggerResources(client *http.Client, blogId string, resources []BloggerResource) ([]BloggerResource, error) {
	service, err := blogger.New(client)
	if err != nil {
		return nil, err
	}

	// Check blogId
	blogCall := service.Blogs.Get(blogId)
	blog, err := blogCall.Do()
	if err != nil {
		return nil, err
	}

	// Find AlbumId
	albumId, err := FindAlbumId(client, blog.Name)
	if err != nil {
		return nil, err
	}

	// Select host
	host := BlogspotHosts[rand.Int31n(int32(len(BlogspotHosts)))]

	// Upload
	output := make([]BloggerResource, 0, len(resources))
	for _, resource := range resources {

		filename := filepath.Base(resource.FileName)
		if ext := filepath.Ext(filename); len(ext) <= 0 {
			filename = filename + getExt(resource.Mime)
		} else if ext != getExt(resource.Mime) {
			filename = filename[:len(filename)-len(ext)] + getExt(resource.Mime)
		}

		photo, err := picago.UploadPhoto(
			client,
			"",
			albumId,
			filename,
			resource.Summary,
			resource.Mime,
			resource.Raws,
		)
		if err != nil {
			return nil, err
		}

		u, err := url.Parse(photo.URL)
		if err != nil {
			return nil, err
		}

		lastSlash := strings.LastIndex(u.Path, "/")
		u.Scheme = "https"
		u.Host = host
		u.Path = u.Path[0:lastSlash+1] + "s1600" + u.Path[lastSlash:]

		res := resource
		res.URL = u.String()
		output = append(output, res)
	}

	return output, nil
}

func PostBloggerPost(client *http.Client, blogId, title, content string, lables []string, draft bool) (*blogger.Post, error) {
	service, err := blogger.New(client)
	if err != nil {
		return nil, err
	}

	post := &blogger.Post{
		Content: content,
		Labels:  lables,
		Title:   title,
	}

	rpost, err := service.Posts.Insert(blogId, post).IsDraft(draft).Do()
	if err != nil {
		return nil, err
	}
	return rpost, nil
}
