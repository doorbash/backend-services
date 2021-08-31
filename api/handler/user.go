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

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	_, err := u.repo.GetByEmail(ctx, authUser.Email)
	if err != nil {
		log.Println("email address", authUser.Email, "is admin but there is no record in database for it.")
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	body, ok := jsonBody.(map[string]interface{})
	if !ok {
		log.Println("bad json", jsonBody)
		util.WriteStatus(w, http.StatusBadRequest)
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
		log.Println("bad project quota")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	projectQuota := int(quota)
	if projectQuota < 0 || float64(projectQuota) != quota {
		log.Println("bad project quota")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	log.Println("project_quota:", projectQuota)
	if user.ProjectQuota == projectQuota {
		log.Println("nothings changed")
		util.WriteOK(w)
		return
	}
	if projectQuota > 0 && projectQuota < user.NumProjects {
		log.Println("Error: project quota cannot be less than user num projects.")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Error: project quota cannot be less than user num projects.")
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

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	_, err := u.repo.GetByEmail(ctx, authUser.Email)
	if err != nil {
		log.Println("email address", authUser.Email, "is admin but there is no record in database for it.")
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	body, ok := jsonBody.(map[string]interface{})
	if !ok {
		util.WriteError(w, http.StatusBadRequest, "bad json")
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
	projectQuota := int(quota)
	if projectQuota < 0 || float64(projectQuota) != quota {
		util.WriteError(w, http.StatusBadRequest, "bad project quota")
		return
	}
	log.Println("project_quota:", projectQuota)
	user := &domain.User{
		Email:        userEmail,
		ProjectQuota: projectQuota,
	}
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	id, err := u.repo.Insert(ctx, user)
	if err != nil {
		log.Println(err)
		if strings.HasPrefix(err.Error(), "ERROR: duplicate key") {
			util.WriteStatus(w, http.StatusConflict)
		} else {
			util.WriteStatus(w, http.StatusBadRequest)
		}
		return
	}
	util.WriteJson(w, id)
}

func NewUserHandler(r *mux.Router, authMiddleware mux.MiddlewareFunc, repo domain.UserRepository, prefix string) *UserHandler {
	rc := &UserHandler{
		repo:   repo,
		router: r,
	}

	rc.router = r.PathPrefix(prefix).Subrouter()
	rc.router.Use(authMiddleware)
	rc.router.HandleFunc("/profile", rc.UserProfileHandler).Methods("GET")
	rc.router.HandleFunc("/role", rc.UserRoleHandler).Methods("GET")

	subrouter := rc.router.NewRoute().Subrouter()
	subrouter.Use(middleware.JsonBodyMiddleware)
	subrouter.HandleFunc("/update", rc.AdminUpdateUserHandler).Methods("POST")
	subrouter.HandleFunc("/new", rc.AdminAddUserHandler).Methods("POST")
	return rc
}
