package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/doorbash/backend-services/api/util"
	"github.com/doorbash/backend-services/api/util/middleware"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type UserHandler struct {
	repo   domain.UserRepository
	router *mux.Router
}

func (u *UserHandler) UserProfileHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)

	user, err := u.repo.GetByEmail(r.Context(), authUser.Email)
	if err != nil {
		log.Println(err)
		if err == pgx.ErrNoRows {
			util.WriteUnauthorized(w)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}
	util.WriteJson(w, user)
}

func (u *UserHandler) UserRoleHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)

	ret := make(map[string]string)
	ret["email"] = authUser.Email
	ret["admin"] = fmt.Sprintf("%t", authUser.IsAdmin)
	util.WriteJson(w, &ret)
}

func (u *UserHandler) AdminUpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	jsonBody := r.Context().Value("json")

	if !authUser.IsAdmin {
		util.WriteStatus(w, http.StatusForbidden)
		return
	}

	body, ok := jsonBody.(map[string]interface{})
	if !ok {
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	_, err := u.repo.GetByEmail(ctx, authUser.Email)
	if err != nil {
		log.Println("email address", authUser.Email, "is admin but there is no record in database for it.")
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	userEmail, ok := body["email"].(string)
	if !ok || userEmail == "" {
		log.Println("bad user email")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	user, err := u.repo.GetByEmail(ctx, userEmail)
	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	quota, ok := body["project_quota"].(float64)
	if !ok {
		log.Println(err)
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	projectQuota := int(quota)
	log.Println("project_quota:", projectQuota)
	if user.ProjectQuota == projectQuota {
		log.Println("nothings changed")
		util.WriteOK(w)
		return
	}
	if projectQuota > 0 && projectQuota < user.NumProjects {
		log.Println("Error: project quota cannot be less than user num projects.")
		util.WriteError(w, http.StatusBadRequest, "Error: project quota cannot be less than user num projects.")
		return
	}
	user.ProjectQuota = projectQuota
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = u.repo.Update(ctx, user)
	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	util.WriteOK(w)
}

func (u *UserHandler) AdminAddUserHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	jsonBody := r.Context().Value("json")

	if !authUser.IsAdmin {
		util.WriteStatus(w, http.StatusForbidden)
		return
	}

	body, ok := jsonBody.(map[string]interface{})
	if !ok {
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	_, err := u.repo.GetByEmail(ctx, authUser.Email)
	if err != nil {
		log.Println("email address", authUser.Email, "is admin but there is no record in database for it.")
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	userEmail, ok := body["email"].(string)
	if !ok || userEmail == "" {
		log.Println("bad user email")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	quota, ok := body["project_quota"].(float64)
	if !ok {
		util.WriteError(w, http.StatusBadRequest, "bad project quota")
		return
	}
	user := &domain.User{
		Email:        userEmail,
		ProjectQuota: int(quota),
	}
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = u.repo.Insert(ctx, user)
	if err != nil {
		log.Println(err)
		if strings.HasPrefix(err.Error(), "ERROR: duplicate key") {
			util.WriteStatus(w, http.StatusConflict)
		} else {
			util.WriteStatus(w, http.StatusBadRequest)
		}
		return
	}
	util.WriteJson(w, user)
}

func NewUserHandler(r *mux.Router, authMiddleware mux.MiddlewareFunc, repo domain.UserRepository) *UserHandler {
	u := &UserHandler{
		repo:   repo,
		router: r.PathPrefix("/users").Subrouter(),
	}

	u.router.Use(authMiddleware)
	u.router.HandleFunc("/profile", u.UserProfileHandler).Methods("GET")
	u.router.HandleFunc("/role", u.UserRoleHandler).Methods("GET")

	jsonRouter := u.router.NewRoute().Subrouter()
	jsonRouter.Use(middleware.JsonBodyMiddleware)
	jsonRouter.HandleFunc("/update", u.AdminUpdateUserHandler).Methods("POST")
	jsonRouter.HandleFunc("/new", u.AdminAddUserHandler).Methods("POST")

	return u
}
