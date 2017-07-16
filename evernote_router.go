package main

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mrjones/oauth"
	"github.com/twsiyuan/evernote-sdk-golang/edam"
	"github.com/twsiyuan/evernote-sdk-golang/edamutil"
)

func evernoteRouter(config *Config, r *mux.Router) (http.Handler, error) {
	enviroment := edamutil.PRODUCTION
	host := edamutil.Host(enviroment)
	requestToken := (*oauth.RequestToken)(nil)
	client := oauth.NewConsumer(
		config.EvernoteClientKey,
		config.EvernoteClientSecret,
		oauth.ServiceProvider{
			RequestTokenUrl:   fmt.Sprintf("https://%s/oauth", host),
			AuthorizeTokenUrl: fmt.Sprintf("https://%s/OAuth.action", host),
			AccessTokenUrl:    fmt.Sprintf("https://%s/oauth", host),
		},
	)

	userStore, err := edamutil.NewUserStore(enviroment)
	if err != nil {
		return nil, err
	}

	noteStoreLock := sync.Mutex{}
	noteStore := (*edam.NoteStoreClient)(nil)
	if len(config.EvernoteAccessToken) > 0 {
		noteStore, err = edamutil.NewNoteStore(userStore, config.EvernoteAccessToken)
		if err != nil {
			if nerr, ok := err.(*edam.EDAMUserException); ok {
				if nerr.ErrorCode == edam.EDAMErrorCode_AUTH_EXPIRED || nerr.ErrorCode == edam.EDAMErrorCode_INVALID_AUTH {
					config.EvernoteAccessToken = ""
					noteStore = nil
				} else {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	authMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if r := recover(); r != nil {
					if err, ok := r.(*edam.EDAMUserException); ok {
						if err.ErrorCode == edam.EDAMErrorCode_AUTH_EXPIRED || err.ErrorCode == edam.EDAMErrorCode_INVALID_AUTH {
							config.EvernoteAccessToken = ""
							userStore = nil
							http.Redirect(w, req, "/evernote/login", http.StatusTemporaryRedirect)
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

	r.HandleFunc("/login", func(w http.ResponseWriter, req *http.Request) {

		reqToken, url, err := client.GetRequestTokenAndUrl("http://127.0.0.1" + config.Addr + "/evernote/login/callback")
		if err != nil {
			panic(err)
		}

		requestToken = reqToken
		http.Redirect(w, req, url, http.StatusTemporaryRedirect)
	})

	r.HandleFunc("/login/callback", func(w http.ResponseWriter, req *http.Request) {
		if requestToken == nil {
			http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
			return
		}

		verifier := ""
		values := req.URL.Query()
		if token, ok := values["oauth_token"]; ok && token[0] == requestToken.Token {
			if ver, ok := values["oauth_verifier"]; ok {
				verifier = ver[0]
			}
		}

		if len(verifier) > 0 {
			authorizedToken, err := client.AuthorizeToken(requestToken, verifier)
			if err != nil {
				panic(err)
			}

			config.EvernoteAccessToken = authorizedToken.Token
			config.Save()

			noteStore, err = edamutil.NewNoteStore(userStore, config.EvernoteAccessToken)
			if err != nil {
				panic(err)
			}

			authorizedToken = nil

			http.Redirect(w, req, "/", http.StatusTemporaryRedirect)

		} else {
			fmt.Fprint(w, "BadRequest")
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	ar := r.PathPrefix("/api").Subrouter()

	ar.HandleFunc("/notebooks", authMiddleware(func(w http.ResponseWriter, req *http.Request) {
		noteStoreLock.Lock()
		defer noteStoreLock.Unlock()
		list, err := noteStore.ListNotebooks(config.EvernoteAccessToken)
		if err != nil {
			panic(err)
		}

		t := make([]noteItem, 0, len(list))
		for _, l := range list {
			t = append(t, noteItem{
				l.GetName(),
				string(l.GetGUID()),
			})
		}

		sort.SliceStable(t, func(i int, k int) bool {
			return t[i].Name < t[k].Name
		})

		renderer.JSON(w, http.StatusOK, t)
	}))

	ar.HandleFunc("/notebooks/{GUID:[a-fA-f0-9\\-]+}", authMiddleware(func(w http.ResponseWriter, req *http.Request) {
		guid := edam.GUID(mux.Vars(req)["GUID"])
		maxNotes := int32(50)

		or := int32(edam.NoteSortOrder_UPDATED)
		filter := edam.NewNoteFilter()
		filter.NotebookGuid = &guid
		filter.Order = &or

		it := true
		resultSpec := edam.NewNotesMetadataResultSpec()
		resultSpec.IncludeTitle = &it

		noteStoreLock.Lock()
		defer noteStoreLock.Unlock()
		result, err := noteStore.FindNotesMetadata(config.EvernoteAccessToken, filter, 0, maxNotes, resultSpec)
		if err != nil {
			panic(err)
		}

		notes := result.GetNotes()
		n := make([]noteItem, 0, len(notes))
		for _, note := range notes {
			n = append(n, noteItem{
				note.GetTitle(),
				string(note.GetGUID()),
			})
		}

		renderer.JSON(w, http.StatusOK, n)
	}))

	ar.HandleFunc("/notes/{GUID:[a-fA-f0-9\\-]+}", authMiddleware(func(w http.ResponseWriter, req *http.Request) {
		guid := edam.GUID(mux.Vars(req)["GUID"])

		noteStoreLock.Lock()
		defer noteStoreLock.Unlock()
		note, err := noteStore.GetNote(config.EvernoteAccessToken, guid, true, false, false, false)
		if err != nil {
			panic(err)
		}

		hash2guid := make(map[string]string)
		for _, res := range note.GetResources() {
			guid := string(res.GetGUID())
			hash := hex.EncodeToString(res.GetData().GetBodyHash())
			hash2guid[hash] = guid
		}

		title := note.GetTitle()
		content, _ := NoteToMarkdown(note.GetContent(), hash2guid)
		tags := note.GetTagNames()

		renderer.JSON(w, http.StatusOK, struct {
			Title   string
			Content string
			Tags    []string
		}{
			title,
			content,
			tags,
		})
	}))

	resourceCache := make(map[string]noteResourceCache)

	ar.HandleFunc("/resources/{GUID:[a-fA-f0-9\\-]+}", authMiddleware(func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		guid := edam.GUID(vars["GUID"])

		noteStoreLock.Lock()
		defer noteStoreLock.Unlock()

		res, err := noteStore.GetResource(config.EvernoteAccessToken, guid, false, false, false, false)
		if err != nil {
			if userErr, ok := err.(*edam.EDAMUserException); ok {
				if userErr.ErrorCode == edam.EDAMErrorCode_BAD_DATA_FORMAT {
					w.WriteHeader(http.StatusNotFound)
					return
				}
			}
			panic(err)
		}

		note := string(res.GetNoteGuid())
		hash := hex.EncodeToString(res.GetData().GetBodyHash())

		u := *req.URL
		u.Path = "/evernote/api/notes/" + note + "/resources/" + hash

		http.Redirect(w, req, u.String(), http.StatusPermanentRedirect)
	}))

	ar.HandleFunc("/notes/{GUID:[a-fA-f0-9\\-]+}/resources/{hash:[0-9a-fA-F]{32}}", authMiddleware(func(w http.ResponseWriter, req *http.Request) {

		vars := mux.Vars(req)
		guid := edam.GUID(vars["GUID"])
		hashHex := vars["hash"]
		hash, err := hex.DecodeString(hashHex)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var body []byte
		var contentType string

		if cache, ok := resourceCache[hashHex]; ok {
			body = cache.Body
			contentType = cache.Type
		} else {
			noteStoreLock.Lock()
			defer noteStoreLock.Unlock()
			res, err := noteStore.GetResourceByHash(config.EvernoteAccessToken, guid, hash, true, false, false)
			if err != nil {
				if _, ok := err.(*edam.EDAMNotFoundException); ok {
					w.WriteHeader(http.StatusNotFound)
					return
				} else {
					panic(err)
				}
			}

			body = res.GetData().GetBody()
			contentType = res.GetMime()

			cache := noteResourceCache{
				Body: body,
				Type: contentType,
			}

			resourceCache[hashHex] = cache
		}

		w.Header().Set("CONTENT-TYPE", contentType)
		w.Write(body)

	}))

	return r, nil
}

type noteItem struct {
	Name string
	GUID string
}

type noteResourceCache struct {
	Body []byte
	Type string
}
