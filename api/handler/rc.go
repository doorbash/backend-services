package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/doorbash/backend-services-api/api/domain"
	"github.com/doorbash/backend-services-api/api/util"
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
	projectVar, ok := mux.Vars(r)["id"]
	if !ok {
		util.WriteInternalServerError(w)
		return
	}
	var remoteConfig *domain.RemoteConfig
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	data, err := rc.rcCache.GetDataByProjectID(ctx, projectVar)
	if err != nil {
		log.Println(err)
		if err == redis.Nil {
			// nothing in cache. get data from db then save to cache
			ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			remoteConfig, err = rc.rcRepo.GetByProjectID(ctx, projectVar)
			if err != nil {
				if err == pgx.ErrNoRows {
					util.WriteStatus(w, http.StatusNotFound)
				} else {
					util.WriteInternalServerError(w)
				}
				return
			}
			ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			err = rc.rcCache.Update(ctx, remoteConfig)
			if err != nil {
				log.Println(err)
				util.WriteInternalServerError(w)
				return
			}
			util.WriteJsonRaw(w, remoteConfig.Data)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}
	util.WriteJsonRaw(w, data)
}

func (rc *RemoteConfigHandler) UpdateDataHandler(w http.ResponseWriter, r *http.Request, jsonBody *interface{}) {
	projectVar, ok := mux.Vars(r)["id"]
	if !ok {
		log.Println("no id")
		util.WriteInternalServerError(w)
		return
	}
	var project *domain.Project
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	project, err := rc.prRepo.GetByID(ctx, projectVar)
	if err != nil {
		log.Println(err)
		if err == pgx.ErrNoRows {
			util.WriteStatus(w, http.StatusNotFound)
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}
	data, err := json.Marshal(jsonBody)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}
	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	remoteConfig, err := rc.rcRepo.GetByProjectID(ctx, project.ID)
	if err != nil {
		if err == pgx.ErrNoRows {
			remoteConfig := &domain.RemoteConfig{
				ProjectID: project.ID,
				Data:      string(data),
			}
			log.Println(remoteConfig)
			ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			err = rc.rcRepo.Insert(ctx, remoteConfig)
			if err != nil {
				log.Println(err)
				util.WriteInternalServerError(w)
				return
			}
			util.WriteOK(w)
		} else {
			log.Println(err)
			util.WriteInternalServerError(w)
		}
		return
	}
	remoteConfig.Data = string(data)
	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err = rc.rcRepo.Update(ctx, remoteConfig)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}
	ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	// update cache
	err = rc.rcCache.Update(ctx, remoteConfig)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}
	util.WriteOK(w)
}

func NewRemoteConfigHandler(
	r *mux.Router,
	authMiddleware mux.MiddlewareFunc,
	rcRepo domain.RemoteConfigRepository,
	prRepo domain.ProjectRepository,
	rcCache domain.RemoteConfigCache,
	prefix string,
) *RemoteConfigHandler {

	rc := &RemoteConfigHandler{
		rcCache: rcCache,
		rcRepo:  rcRepo,
		prRepo:  prRepo,
		router:  r,
	}

	rc.router = r.PathPrefix(prefix).Subrouter()
	rc.router.HandleFunc("/{id}/", rc.GetDataHandler).Methods("GET")

	authRouter := rc.router.NewRoute().Subrouter()
	authRouter.Use(authMiddleware)
	authRouter.HandleFunc("/{id}/", util.JsonBodyMiddleware(rc.UpdateDataHandler).ServeHTTP).Methods("POST")

	return rc
}
