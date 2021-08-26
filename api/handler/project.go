package handler

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/doorbash/backend-services-api/api/domain"
	"github.com/doorbash/backend-services-api/api/util"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type ProjectHandler struct {
	prRepo   domain.ProjectRepository
	userRepo domain.UserRepository
	router   *mux.Router
}

func (pr *ProjectHandler) GetAllProjectsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	user, err := pr.userRepo.GetByEmail(ctx, r.Header.Get("email"))
	if err != nil {
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	projects, err := pr.prRepo.GetProjectsByUserID(ctx, user.ID)
	if err != nil {
		util.WriteInternalServerError(w)
		return
	}
	util.WriteJson(w, projects)
}

func (pr *ProjectHandler) GetProjectHandler(w http.ResponseWriter, r *http.Request) {
	projectVar, ok := mux.Vars(r)["id"]
	if !ok {
		util.WriteInternalServerError(w)
		return
	}
	var project *domain.Project
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	project, err := pr.prRepo.GetByID(ctx, projectVar)
	if err != nil {
		if err == pgx.ErrNoRows {
			util.WriteStatus(w, http.StatusNotFound)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}
	util.WriteJson(w, project)
}

func (pr *ProjectHandler) CreateProjectHandler(w http.ResponseWriter, r *http.Request, jsonBody *interface{}) {
	body, ok := (*jsonBody).(map[string]interface{})
	if !ok {
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	name, ok := body["name"].(string)
	if !ok {
		log.Println("no name")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	id, ok := body["id"].(string)
	if !ok {
		log.Println("no id")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	user, err := pr.userRepo.GetByEmail(ctx, r.Header.Get("email"))
	if err != nil {
		if err == pgx.ErrNoRows {
			util.WriteUnauthorized(w)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}
	project := &domain.Project{
		ID:     id,
		UserID: user.ID,
		Name:   name,
	}
	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err = pr.prRepo.Insert(ctx, project)
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

func (pr *ProjectHandler) UpdateProjectHandler(w http.ResponseWriter, r *http.Request, jsonBody *interface{}) {
	body, ok := (*jsonBody).(map[string]interface{})
	if !ok {
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	name, ok := body["name"].(string)
	if !ok {
		log.Println("no name")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	id, ok := body["id"].(string)
	if !ok {
		log.Println("no id")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_, err := pr.userRepo.GetByEmail(ctx, r.Header.Get("email"))
	if err != nil {
		if err == pgx.ErrNoRows {
			util.WriteUnauthorized(w)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}
	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	project, err := pr.prRepo.GetByID(ctx, id)
	if err != nil {
		log.Println(err)
		if err == pgx.ErrNoRows {
			util.WriteStatus(w, http.StatusNotFound)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}

	if project.Name == name {
		util.WriteOK(w)
		return
	}

	project.Name = name

	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err = pr.prRepo.Update(ctx, project)
	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	util.WriteOK(w)
}

func (pr *ProjectHandler) DeleteProjectHandler(w http.ResponseWriter, r *http.Request, jsonBody *interface{}) {
	body, ok := (*jsonBody).(map[string]interface{})
	if !ok {
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	id, ok := body["id"].(string)
	if !ok {
		log.Println("no id")
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	user, err := pr.userRepo.GetByEmail(ctx, r.Header.Get("email"))
	if err != nil {
		if err == pgx.ErrNoRows {
			util.WriteUnauthorized(w)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}

	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	project, err := pr.prRepo.GetByID(ctx, id)
	if err != nil {
		log.Println(err)
		if err == pgx.ErrNoRows {
			util.WriteStatus(w, http.StatusNotFound)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}

	if user.ID != project.UserID {
		util.WriteStatus(w, http.StatusForbidden)
		return
	}

	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err = pr.prRepo.Delete(ctx, project)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}
	util.WriteOK(w)
}

func NewProjectHandler(r *mux.Router, authMiddleware mux.MiddlewareFunc, prRepo domain.ProjectRepository, userRepo domain.UserRepository, prefix string) *ProjectHandler {
	rc := &ProjectHandler{
		prRepo:   prRepo,
		userRepo: userRepo,
		router:   r,
	}
	rc.router = r.PathPrefix(prefix).Subrouter()
	rc.router.Use(authMiddleware)
	rc.router.HandleFunc("/", rc.GetAllProjectsHandler).Methods("GET")
	rc.router.HandleFunc("/{id}/", rc.GetProjectHandler).Methods("GET")
	rc.router.HandleFunc("/new", util.JsonBodyMiddleware(rc.CreateProjectHandler).ServeHTTP).Methods("POST")
	rc.router.HandleFunc("/delete", util.JsonBodyMiddleware(rc.DeleteProjectHandler).ServeHTTP).Methods("POST")
	rc.router.HandleFunc("/update", util.JsonBodyMiddleware(rc.UpdateProjectHandler).ServeHTTP).Methods("POST")
	return rc
}
