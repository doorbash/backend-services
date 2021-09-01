package auth

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/doorbash/backend-services/api/util"
	"github.com/doorbash/backend-services/api/util/middleware"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v4"
	"golang.org/x/oauth2"
)

const (
	githubAuthorizeUrl = "https://github.com/login/oauth/authorize"
	githubTokenUrl     = "https://github.com/login/oauth/access_token"
)

type GithubOAuth2Handler struct {
	store        *sessions.CookieStore
	oauthCfg     *oauth2.Config
	router       *mux.Router
	userRepo     domain.UserRepository
	authCache    domain.AuthCache
	clientSecret string
	clientID     string
	sessionKey   string
	admin        string
	apiPath      string
	isPrivate    bool
	prefix       string
}

func (o *GithubOAuth2Handler) Middleware(h http.Handler) http.Handler {
	return middleware.OAuth2Middleware(o.authCache, o.admin, h)
}

func (o *GithubOAuth2Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 16)
	rand.Read(b)

	state := base64.URLEncoding.EncodeToString(b)

	session, _ := o.store.Get(r, SESSION_STORE_KEY)
	session.Values["state"] = state
	session.Save(r, w)

	url := o.oauthCfg.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}

func (o *GithubOAuth2Handler) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	session, err := o.store.Get(r, SESSION_STORE_KEY)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "Aborted")
		return
	}

	if r.URL.Query().Get("state") != session.Values["state"] {
		util.WriteError(w, http.StatusBadRequest, "No state match; possible csrf OR cookies not enabled")
		return
	}

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	token, err := o.oauthCfg.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		log.Println(err)
		util.WriteError(w, http.StatusBadRequest, "There was an issue getting your token")
		return
	}

	if !token.Valid() {
		util.WriteError(w, http.StatusBadRequest, "Retreived invalid token")
		return
	}

	client := github.NewClient(o.oauthCfg.Client(r.Context(), token))

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	githubUser, _, err := client.Users.Get(ctx, "")

	if err != nil || githubUser == nil || githubUser.Email == nil {
		if err != nil {
			log.Println(err)
		}
		util.WriteError(w, http.StatusBadRequest, "Error getting email from github. Please make sure you have set your email as Public email in Github settings.")
		return
	}

	email := *githubUser.Email
	var id int

	ctx, cancel = util.GetContextWithTimeout(context.Background())
	defer cancel()
	user, err := o.userRepo.GetByEmail(ctx, email)

	if err != nil {
		if err == pgx.ErrNoRows {
			// no record in database for this user
			if email != o.admin && o.isPrivate {
				log.Printf("login from %s but API is private\n", email)
				util.WriteError(w, http.StatusForbidden, "This API is private. Please contact administrator.")
				return
			}
			ctx, cancel = util.GetContextWithTimeout(context.Background())
			defer cancel()
			id, err = o.userRepo.Insert(ctx, &domain.User{Email: email, ProjectQuota: 0})
			if err != nil {
				log.Println(err)
				util.WriteInternalServerError(w)
				return
			}
		} else {
			log.Println(err)
			util.WriteInternalServerError(w)
			return
		}
	} else {
		id = user.ID
	}

	session.Values["email"] = email
	session.Values["id"] = id
	err = sessions.Save(r, w)

	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}

	url, _ := o.router.Get("credentials").URL()

	http.Redirect(w, r, fmt.Sprintf("%s%s", o.apiPath, url.Path), http.StatusFound)
}

func (o *GithubOAuth2Handler) CredentialsHandler(w http.ResponseWriter, r *http.Request) {
	session, err := o.store.Get(r, SESSION_STORE_KEY)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "Aborted")
		return
	}
	email, ok := session.Values["email"].(string)
	if !ok {
		log.Println("no email")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	id, ok := session.Values["id"].(int)
	if !ok {
		log.Println("no id")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	ctx, cancel := util.GetContextWithTimeout(context.Background())
	defer cancel()
	t, err := o.authCache.GenerateAndSaveToken(ctx, email, id)

	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}

	session.Options.MaxAge = -1

	err = session.Save(r, w)

	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}

	authToken := &domain.AuthToken{
		AccessToken: t,
		TokenType:   "bearer",
		ExpiresIn:   o.authCache.GetTokenExpiry(),
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	util.WriteJson(w, authToken)
}

func (o *GithubOAuth2Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("token")
	ctx, cancel := util.GetContextWithTimeout(context.Background())
	defer cancel()
	err := o.authCache.DeleteToken(ctx, token)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}
	util.WriteOK(w)
}

func NewGithubOAuth2Handler(
	r *mux.Router,
	userRepo domain.UserRepository,
	authCache domain.AuthCache,
	clientSecret string,
	clientID string,
	sessionKey string,
	admin string,
	apiPath string,
	isPrivate bool,
	prefix string,
) *GithubOAuth2Handler {
	rc := &GithubOAuth2Handler{
		store: sessions.NewCookieStore([]byte(sessionKey)),
		oauthCfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  githubAuthorizeUrl,
				TokenURL: githubTokenUrl,
			},
			Scopes: []string{},
		},
		router:       r,
		userRepo:     userRepo,
		authCache:    authCache,
		clientSecret: clientSecret,
		clientID:     clientID,
		sessionKey:   sessionKey,
		admin:        admin,
		apiPath:      apiPath,
		prefix:       prefix,
	}

	rc.router = r.PathPrefix(prefix).Subrouter()
	rc.router.HandleFunc("/login", rc.LoginHandler).Methods("GET")
	rc.router.HandleFunc("/callback", rc.CallbackHandler).Methods("GET")
	rc.router.HandleFunc("/credentials", rc.CredentialsHandler).Methods("GET").Name("credentials")
	rc.router.HandleFunc("/logout", rc.Middleware(http.HandlerFunc(rc.LogoutHandler)).ServeHTTP).Methods("GET")

	return rc
}
