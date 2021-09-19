package auth

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

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
	redirectPath := r.URL.Query().Get("redirect_path")

	b := make([]byte, 16)
	rand.Read(b)

	state := base64.URLEncoding.EncodeToString(b)

	session, _ := o.store.Get(r, SESSION_STORE_KEY)
	session.Values["state"] = state
	if redirectPath != "" {
		u, err := url.Parse(redirectPath)
		if err != nil {
			log.Println(err)
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad redirect_path: %s", redirectPath))
			return
		}
		if u.Scheme != "" || u.Host != "" {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("redirect_path must be relative: %s", redirectPath))
			return
		}

		session.Values["redirect_path"] = redirectPath
	} else {
		session.Values["redirect_path"] = ""
	}
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

	redirectPath, _ := session.Values["redirect_path"].(string)

	if r.URL.Query().Get("state") != session.Values["state"] {
		e := "No state match; possible csrf OR cookies not enabled"
		log.Println(e)
		if redirectPath == "" {
			util.WriteError(w, http.StatusBadRequest, e)
		} else {
			http.Redirect(
				w,
				r,
				fmt.Sprintf(
					"%s?error=%s",
					redirectPath,
					url.QueryEscape(e),
				),
				http.StatusFound,
			)
		}
		return
	}

	http.DefaultTransport.(*http.Transport).DialContext = (&net.Dialer{
		Timeout: 10 * time.Second,
	}).DialContext
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	token, err := o.oauthCfg.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		log.Println(err)
		e := "There was an issue getting your token"
		log.Println(e)
		if redirectPath == "" {
			util.WriteError(w, http.StatusBadRequest, e)
		} else {
			http.Redirect(
				w,
				r,
				fmt.Sprintf(
					"%s?error=%s",
					redirectPath,
					url.QueryEscape(e),
				),
				http.StatusFound,
			)
		}
		return
	}

	if !token.Valid() {
		e := "Retreived invalid token"
		log.Println(e)
		if redirectPath == "" {
			util.WriteError(w, http.StatusBadRequest, e)
		} else {
			http.Redirect(
				w,
				r,
				fmt.Sprintf(
					"%s?error=%s",
					redirectPath,
					url.QueryEscape(e),
				),
				http.StatusFound,
			)
		}
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
		e := "Error getting email from github. Please make sure you have set your email as Public email in Github settings."
		log.Println(e)
		if redirectPath == "" {
			util.WriteError(w, http.StatusBadRequest, e)
		} else {
			http.Redirect(
				w,
				r,
				fmt.Sprintf(
					"%s?error=%s",
					redirectPath,
					url.QueryEscape(e),
				),
				http.StatusFound,
			)
		}
		return
	}

	email := *githubUser.Email

	ctx, cancel = util.GetContextWithTimeout(context.Background())
	defer cancel()
	user, err := o.userRepo.GetByEmail(ctx, email)

	if err != nil {
		if err == pgx.ErrNoRows {
			// no record in database for this user
			if email != o.admin && o.isPrivate {
				e := "This API is private. Please contact administrator."
				log.Println(e)
				if redirectPath == "" {
					util.WriteError(w, http.StatusForbidden, e)
				} else {
					http.Redirect(
						w,
						r,
						fmt.Sprintf(
							"%s?error=%s",
							redirectPath,
							url.QueryEscape(e),
						),
						http.StatusFound,
					)
				}
				return
			}
			ctx, cancel = util.GetContextWithTimeout(context.Background())
			defer cancel()
			user = &domain.User{Email: email, ProjectQuota: 0}
			err = o.userRepo.Insert(ctx, user)
			if err != nil {
				log.Println(err)

				if redirectPath == "" {
					util.WriteInternalServerError(w)
				} else {
					http.Redirect(
						w,
						r,
						fmt.Sprintf(
							"%s?error=%s",
							redirectPath,
							url.QueryEscape(http.StatusText(http.StatusInternalServerError)),
						),
						http.StatusFound,
					)
				}
				return
			}
		} else {
			log.Println(err)

			if redirectPath == "" {
				util.WriteInternalServerError(w)
			} else {
				http.Redirect(
					w,
					r,
					fmt.Sprintf(
						"%s?error=%s",
						redirectPath,
						url.QueryEscape(http.StatusText(http.StatusInternalServerError)),
					),
					http.StatusFound,
				)
			}
			return
		}
	}

	session.Values["email"] = user.Email
	session.Values["id"] = user.ID
	err = sessions.Save(r, w)

	if err != nil {
		log.Println(err)

		if redirectPath == "" {
			util.WriteInternalServerError(w)
		} else {
			http.Redirect(
				w,
				r,
				fmt.Sprintf(
					"%s?error=%s",
					redirectPath,
					url.QueryEscape(http.StatusText(http.StatusInternalServerError)),
				),
				http.StatusFound,
			)
		}
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

	redirectPath, _ := session.Values["redirect_path"].(string)

	email, ok := session.Values["email"].(string)
	if !ok {
		e := "no email"
		log.Println(e)
		if redirectPath == "" {
			util.WriteError(w, http.StatusBadRequest, e)
		} else {
			http.Redirect(
				w,
				r,
				fmt.Sprintf(
					"%s?error=%s",
					redirectPath,
					url.QueryEscape(e),
				),
				http.StatusFound,
			)
		}
		return
	}
	id, ok := session.Values["id"].(int)
	if !ok {
		e := "no id"
		log.Println(e)
		if redirectPath == "" {
			util.WriteError(w, http.StatusBadRequest, e)
		} else {
			http.Redirect(
				w,
				r,
				fmt.Sprintf(
					"%s?error=%s",
					redirectPath,
					url.QueryEscape(e),
				),
				http.StatusFound,
			)
		}
		return
	}

	ctx, cancel := util.GetContextWithTimeout(context.Background())
	defer cancel()
	t, err := o.authCache.GenerateAndSaveToken(ctx, email, id)

	if err != nil {
		log.Println(err)

		if redirectPath == "" {
			util.WriteInternalServerError(w)
		} else {
			http.Redirect(
				w,
				r,
				fmt.Sprintf(
					"%s?error=%s",
					redirectPath,
					url.QueryEscape(http.StatusText(http.StatusInternalServerError)),
				),
				http.StatusFound,
			)
		}
		return
	}

	session.Options.MaxAge = -1

	err = session.Save(r, w)

	if err != nil {
		log.Println(err)

		if redirectPath == "" {
			util.WriteInternalServerError(w)
		} else {
			http.Redirect(
				w,
				r,
				fmt.Sprintf(
					"%s?error=%s",
					redirectPath,
					url.QueryEscape(http.StatusText(http.StatusInternalServerError)),
				),
				http.StatusFound,
			)
		}
		return
	}

	authToken := &domain.AuthToken{
		AccessToken: t,
		TokenType:   "bearer",
		ExpiresIn:   o.authCache.GetTokenExpiry(),
	}

	if redirectPath == "" {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		util.WriteJson(w, authToken)
	} else {
		http.SetCookie(w, &http.Cookie{
			Name:   "access_token",
			Value:  authToken.AccessToken,
			MaxAge: int(authToken.ExpiresIn.Seconds()),
			Path:   "/",
		})
		var role string
		if o.admin == email {
			role = "admin"
		} else {
			role = "member"
		}
		http.SetCookie(w, &http.Cookie{
			Name:   "role",
			Value:  role,
			MaxAge: int(authToken.ExpiresIn.Seconds()),
			Path:   "/",
		})
		http.Redirect(w, r, redirectPath, http.StatusFound)
	}
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
	isPrivate bool,
	apiPath string,
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
