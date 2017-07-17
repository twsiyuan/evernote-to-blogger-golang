package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/gorilla/mux"
)

func bloggerRouter(config *Config, r *mux.Router) (http.Handler, error) {
	oauthConfig := &oauth2.Config{
		ClientID:     config.GoogleAPIClientID,
		ClientSecret: config.GoogleAPIClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://127.0.0.1" + config.Addr + "/blogger/login/callback",
		Scopes: []string{
			"https://www.googleapis.com/auth/blogger",
			"https://picasaweb.google.com/data/",
		},
	}

	oauthToken := oauth2.Token{
		Expiry:       config.GoogleAPIExpiryDate,
		AccessToken:  config.GoogleAPIAccessToken,
		RefreshToken: config.GoogleAPIRefreshToken,
		TokenType:    "Bearer",
	}

	oauthClient := oauthConfig.Client(context.Background(), &oauthToken)

	loginState := ""
	r.HandleFunc("/login", func(w http.ResponseWriter, req *http.Request) {

		loginState = randStringRunes(20)
		url := oauthConfig.AuthCodeURL(loginState, oauth2.ApprovalForce, oauth2.AccessTypeOffline)

		http.Redirect(w, req, url, http.StatusTemporaryRedirect)
	})

	r.HandleFunc("/login/callback", func(w http.ResponseWriter, req *http.Request) {
		values := req.URL.Query()
		state := values["state"][0]
		if state != loginState {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		code := values["code"][0]
		token, err := oauthConfig.Exchange(context.Background(), code)
		if err != nil {
			panic(err)
		}

		oauthClient = oauthConfig.Client(context.Background(), token)
		config.GoogleAPIAccessToken = token.AccessToken
		config.GoogleAPIRefreshToken = token.RefreshToken
		config.GoogleAPIExpiryDate = token.Expiry
		config.Save()

		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
	})

	authMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if r := recover(); r != nil {
					if err, ok := r.(error); ok {
						if strings.Contains(err.Error(), "token expired and refresh token is not set") ||
							strings.Contains(err.Error(), "401: Invalid Credentials") {
							config.GoogleAPIAccessToken = ""
							http.Redirect(w, req, "/blogger/login", http.StatusTemporaryRedirect)
							return
						}
					}
					panic(r)
				}
			}()

			next.ServeHTTP(w, req)
		})
	}

	if r == nil {
		r = mux.NewRouter()
	}

	ar := r.PathPrefix("/api").Subrouter()
	ar.HandleFunc("/blogs", authMiddleware(func(w http.ResponseWriter, req *http.Request) {
		blogs, err := BlogList(oauthClient)
		if err != nil {
			panic(err)
		}

		t := make([]interface{}, 0, len(blogs))
		for _, blog := range blogs {
			t = append(t, struct {
				Id   string
				Name string
			}{
				blog.Id,
				blog.Name,
			})
		}

		renderer.JSON(w, http.StatusOK, t)
	}))

	s16002s780Regexp := regexp.MustCompile("(<img[^>]*)(\\/s1600\\/)([^>]*>)")
	ar.HandleFunc("/blogs/{blogId:[^/]+}/posts", authMiddleware(func(w http.ResponseWriter, req *http.Request) {
		blogId := mux.Vars(req)["blogId"]
		post := struct {
			Title     string
			Content   string
			Lables    []string
			PngToJpeg bool
		}{}

		defer req.Body.Close()
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			panic(err)
		}

		if len(b) <= 0 {
			println("No content")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := json.Unmarshal(b, &post); err != nil {
			println(err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Read all img src
		baseUrl := req.URL.Host
		if len(baseUrl) <= 0 {
			baseUrl = "127.0.0.1" + config.Addr
		}

		res, err := parseAllImageSources(baseUrl, post.Content)
		if err != nil {
			panic(err)
		}

		if post.PngToJpeg {
			for i := 0; i < len(res); i++ {
				r := res[i]
				if r.Mime == "image/png" {
					r.Mime = "image/jpeg"
					img, err := png.Decode(bytes.NewReader(r.Raws))
					if err != nil {
						continue
					}

					b := bytes.NewBuffer(nil)
					if err := jpeg.Encode(b, img, &jpeg.Options{
						Quality: 95,
					}); err != nil {
						continue
					}
					r.Raws = b.Bytes()
					res[i] = r
				}
			}
		}

		pres, err := UploadBloggerResources(oauthClient, blogId, res)
		if err != nil {
			panic(err)
		}

		for idx, _ := range res {
			post.Content = strings.Replace(post.Content, res[idx].URL, pres[idx].URL, -1)
		}

		post.Content = s16002s780Regexp.ReplaceAllStringFunc(post.Content, func(s string) string {
			ss := s16002s780Regexp.FindStringSubmatch(s)
			return ss[1] + "/s780/" + ss[3]
		})

		bpost, err := PostBloggerPost(oauthClient, blogId, post.Title, post.Content, post.Lables, true)
		if err != nil {
			panic(err)
		}

		renderer.JSON(w, http.StatusOK, struct {
			Url string
		}{
			bpost.Id,
		})

	})).Methods("POST")

	return r, nil
}

func parseAllImageSources(addr string, content string) ([]BloggerResource, error) {
	res := make([]BloggerResource, 0, 10)
	nodes, err := html.ParseFragment(
		strings.NewReader(content),
		&html.Node{
			Type:     html.ElementNode,
			Data:     "body",
			DataAtom: atom.Body,
		},
	)
	if err != nil {
		return nil, err
	}

	var f func(node *html.Node)
	f = func(n *html.Node) {
		if n.Data == "img" && n.DataAtom == atom.Img {
			var src, alt, filename string
			for _, attr := range n.Attr {
				if attr.Key == "src" {
					src = attr.Val
				} else if attr.Key == "alt" {
					alt = attr.Val
				}
			}

			u, err := url.Parse(src)
			if err != nil {
				return
			}
			if len(u.Host) <= 0 {
				u.Host = addr
			}
			if len(u.Scheme) <= 0 {
				u.Scheme = "http"
			}

			filename = path.Base(u.Path)
			if fileq, ok := u.Query()["name"]; ok {
				filename = fileq[0]
			}

			url := u.String()
			req, _ := http.NewRequest(http.MethodGet, url, nil)
			rep, err := http.DefaultClient.Do(req)
			if err != nil {
				return
			}
			defer rep.Body.Close()
			b, _ := ioutil.ReadAll(rep.Body)
			mimetype := rep.Header.Get("CONTENT-TYPE")

			res = append(res, BloggerResource{
				FileName: filename,
				Summary:  alt,
				Mime:     mimetype,
				Raws:     b,
				URL:      src,
			})
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	for _, c := range nodes {
		f(c)
	}

	return res, nil
}
