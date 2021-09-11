package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/doorbash/backend-services/api/util"
	"github.com/doorbash/backend-services/api/util/middleware"
	"github.com/go-redis/redis/v8"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type RemoteConfigHandler struct {
	rcCache domain.RemoteConfigCache
	rcRepo  domain.RemoteConfigRepository
	prRepo  domain.ProjectRepository
	router  *mux.Router
}

func (rc *RemoteConfigHandler) GetDataHandler(w http.ResponseWriter, r *http.Request) {
	pid, ok := mux.Vars(r)["id"]
	if !ok {
		util.WriteInternalServerError(w)
		return
	}
	version := util.GetUrlQueryParam(r, "version")
	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	v, err := rc.rcCache.GetVersionByProjectID(ctx, pid)
	if err != nil {
		log.Println(err)
		if err == redis.Nil {
			ctx, cancel = util.GetContextWithTimeout(r.Context())
			defer cancel()
			remoteConfig, err := rc.rcRepo.GetByProjectID(ctx, pid)
			if err != nil {
				if err == pgx.ErrNoRows {
					util.WriteStatus(w, http.StatusNotFound)
				} else {
					util.WriteInternalServerError(w)
				}
				return
			}
			ctx, cancel = util.GetContextWithTimeout(r.Context())
			defer cancel()
			err = rc.rcCache.Update(ctx, remoteConfig)
			if err != nil {
				log.Println(err)
				util.WriteInternalServerError(w)
				return
			}
			util.WriteJson(w, remoteConfig)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}
	if version != "" {
		vi, err := strconv.Atoi(version)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad version %s", version))
			return
		}
		if vi >= *v {
			util.WriteStatus(w, http.StatusNotFound)
			return
		}
	}
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	data, err := rc.rcCache.GetDataByProjectID(ctx, pid)
	if err != nil {
		log.Println(err)
		if err == redis.Nil {
			util.WriteStatus(w, http.StatusNotFound)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}
	remoteConfig := &domain.RemoteConfig{
		ProjectID: pid,
		Data:      *data,
		Version:   *v,
	}
	util.WriteJson(w, remoteConfig)
}

func (rc *RemoteConfigHandler) UpdateDataHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)

	pid, ok := mux.Vars(r)["id"]
	if !ok {
		log.Println("no id")
		util.WriteInternalServerError(w)
		return
	}

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	project, err := rc.prRepo.GetByID(ctx, pid)
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

	data := r.PostFormValue("data")
	if data == "" {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	remoteConfig, err := rc.rcRepo.GetByProjectID(ctx, project.ID)
	if err != nil {
		if err == pgx.ErrNoRows {
			remoteConfig := &domain.RemoteConfig{
				ProjectID: project.ID,
				Data:      string(data),
			}
			log.Println(remoteConfig)
			ctx, cancel = util.GetContextWithTimeout(r.Context())
			defer cancel()
			err = rc.rcRepo.Insert(ctx, remoteConfig)
			if err != nil {
				log.Println(err)
				util.WriteInternalServerError(w)
				return
			}
			util.WriteJson(w, remoteConfig)
		} else {
			log.Println(err)
			util.WriteInternalServerError(w)
		}
		return
	}
	remoteConfig.Data = string(data)
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = rc.rcRepo.Update(ctx, remoteConfig)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = rc.rcCache.Update(ctx, remoteConfig)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}
	util.WriteJson(w, remoteConfig)
}

func NewRemoteConfigHandler(
	r *mux.Router,
	authMiddleware mux.MiddlewareFunc,
	rcRepo domain.RemoteConfigRepository,
	prRepo domain.ProjectRepository,
	rcCache domain.RemoteConfigCache,
) *RemoteConfigHandler {
	rc := &RemoteConfigHandler{
		rcCache: rcCache,
		rcRepo:  rcRepo,
		prRepo:  prRepo,
		router:  r,
	}
	rc.router.HandleFunc("/{id}/rc", rc.GetDataHandler).Methods("GET")

	authRouter := rc.router.NewRoute().Subrouter()
	authRouter.Use(authMiddleware)
	authRouter.HandleFunc("/{id}/rc", rc.UpdateDataHandler).Methods("POST")

	return rc
}
