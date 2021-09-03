package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/doorbash/backend-services/api/util"
	"github.com/doorbash/backend-services/api/util/middleware"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type ProjectHandler struct {
	prRepo   domain.ProjectRepository
	userRepo domain.UserRepository
	router   *mux.Router
}

func (pr *ProjectHandler) GetAllProjectsHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	user, err := pr.userRepo.GetByEmail(ctx, authUser.Email)
	if err != nil {
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	projects, err := pr.prRepo.GetProjectsByUserID(ctx, user.ID)
	if err != nil {
		util.WriteInternalServerError(w)
		return
	}
	util.WriteJson(w, projects)
}

func (pr *ProjectHandler) GetProjectHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)

	pid, ok := mux.Vars(r)["id"]
	if !ok {
		util.WriteInternalServerError(w)
		return
	}

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	project, err := pr.prRepo.GetByID(ctx, pid)
	if err != nil {
		if err == pgx.ErrNoRows {
			util.WriteStatus(w, http.StatusNotFound)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}

	if project.UserID != authUser.ID {
		util.WriteStatus(w, http.StatusForbidden)
		return
	}

	util.WriteJson(w, project)
}

func (pr *ProjectHandler) CreateProjectHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	jsonBody := r.Context().Value("json")

	body, ok := jsonBody.(map[string]interface{})
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
	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	user, err := pr.userRepo.GetByEmail(ctx, authUser.Email)
	if err != nil {
		if err == pgx.ErrNoRows {
			util.WriteUnauthorized(w)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}
	if user.ProjectQuota != 0 && user.NumProjects >= user.ProjectQuota {
		util.WriteError(w, http.StatusForbidden, "Sorry, your quota has been exceeded.")
		return
	}
	project := &domain.Project{
		ID:     id,
		UserID: user.ID,
		Name:   name,
	}
	ctx, cancel = util.GetContextWithTimeout(r.Context())
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

func (pr *ProjectHandler) UpdateProjectHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	jsonBody := r.Context().Value("json")

	pid, ok := mux.Vars(r)["id"]
	if !ok {
		util.WriteInternalServerError(w)
		return
	}

	body, ok := jsonBody.(map[string]interface{})
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
	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	_, err := pr.userRepo.GetByEmail(ctx, authUser.Email)
	if err != nil {
		if err == pgx.ErrNoRows {
			util.WriteUnauthorized(w)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	project, err := pr.prRepo.GetByID(ctx, pid)
	if err != nil {
		log.Println(err)
		if err == pgx.ErrNoRows {
			util.WriteStatus(w, http.StatusNotFound)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}

	if project.UserID != authUser.ID {
		util.WriteStatus(w, http.StatusForbidden)
		return
	}

	if project.Name == name {
		util.WriteOK(w)
		return
	}

	project.Name = name

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = pr.prRepo.Update(ctx, project)
	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	util.WriteOK(w)
}

func (pr *ProjectHandler) DeleteProjectHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)

	pid, ok := mux.Vars(r)["id"]
	if !ok {
		util.WriteInternalServerError(w)
		return
	}

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	project, err := pr.prRepo.GetByID(ctx, pid)
	if err != nil {
		log.Println(err)
		if err == pgx.ErrNoRows {
			util.WriteStatus(w, http.StatusNotFound)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}

	if project.UserID != authUser.ID {
		util.WriteStatus(w, http.StatusForbidden)
		return
	}

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = pr.prRepo.Delete(ctx, project)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}
	util.WriteOK(w)
}

func NewProjectHandler(r *mux.Router, authMiddleware mux.MiddlewareFunc, prRepo domain.ProjectRepository, userRepo domain.UserRepository) *ProjectHandler {
	p := &ProjectHandler{
		prRepo:   prRepo,
		userRepo: userRepo,
		router:   r.NewRoute().Subrouter(),
	}

	p.router.Use(authMiddleware)
	p.router.HandleFunc("/projects", p.GetAllProjectsHandler).Methods("GET")
	p.router.HandleFunc("/{id}/", p.GetProjectHandler).Methods("GET")
	p.router.HandleFunc("/{id}/delete", p.DeleteProjectHandler).Methods("POST")

	subrouter := p.router.NewRoute().Subrouter()
	subrouter.Use(middleware.JsonBodyMiddleware)
	subrouter.HandleFunc("/projects/new", p.CreateProjectHandler).Methods("POST")
	subrouter.HandleFunc("/{id}/update", p.UpdateProjectHandler).Methods("POST")
	return p
}
