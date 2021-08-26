package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/doorbash/backend-services-api/api/domain"
	"github.com/doorbash/backend-services-api/api/util"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type UserHandler struct {
	repo   domain.UserRepository
	router *mux.Router
}

func (u *UserHandler) UserProfileHandler(w http.ResponseWriter, r *http.Request) {
	user, err := u.repo.GetByEmail(r.Context(), r.Header.Get("email"))
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
	ret := make(map[string]string)
	ret["email"] = r.Header.Get("email")
	ret["role"] = r.Header.Get("role")
	util.WriteJson(w, &ret)
}

func (u *UserHandler) AdminUpdateUserHandler(w http.ResponseWriter, r *http.Request, jsonBody *interface{}) {
	email := r.Header.Get("email")
	role := r.Header.Get("role")
	if role != "admin" {
		util.WriteStatus(w, http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_, err := u.repo.GetByEmail(ctx, email)
	if err != nil {
		log.Println("email address", email, "is admin but there is no record in database for it.")
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	body, ok := (*jsonBody).(map[string]interface{})
	if !ok {
		log.Println("bad json")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	userEmail, ok := body["email"].(string)
	if !ok || userEmail == "" {
		log.Println("bad user email")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
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
	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err = u.repo.Update(ctx, user)
	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	util.WriteOK(w)
}

func (u *UserHandler) AdminAddUserHandler(w http.ResponseWriter, r *http.Request, jsonBody *interface{}) {
	email := r.Header.Get("email")
	role := r.Header.Get("role")
	if role != "admin" {
		util.WriteStatus(w, http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_, err := u.repo.GetByEmail(ctx, email)
	if err != nil {
		log.Println("email address", email, "is admin but there is no record in database for it.")
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	body, ok := (*jsonBody).(map[string]interface{})
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
	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
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
	util.WriteOK(w)
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
	rc.router.HandleFunc("/update", util.JsonBodyMiddleware(rc.AdminUpdateUserHandler).ServeHTTP).Methods("POST")
	rc.router.HandleFunc("/new", util.JsonBodyMiddleware(rc.AdminAddUserHandler).ServeHTTP).Methods("POST")
	return rc
}
