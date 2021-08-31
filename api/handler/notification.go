package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/doorbash/backend-services/api/util"
	"github.com/doorbash/backend-services/api/util/middleware"
	"github.com/jackc/pgx/v4"

	"github.com/gorilla/mux"
)

type NotificationHandler struct {
	noCache domain.NotificationCache
	noRepo  domain.NotificationRepository
	prRepo  domain.ProjectRepository
	router  *mux.Router
}

func (n *NotificationHandler) GetNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	projectVar := mux.Vars(r)["id"]
	t := util.GetUrlQueryParam(r, "time")
	var _time time.Time
	if t == "" {
		_time = time.Unix(0, 0)
	} else {
		var err error
		_time, err = time.Parse(time.RFC3339, t)
		if err != nil {
			log.Println(err)
			util.WriteError(w, http.StatusBadRequest, "Bad time")
			return
		}
	}
	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	pt, err := n.noCache.GetTimeByProjectID(ctx, projectVar)
	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	if _time.After(pt) {
		log.Println("_time >= pt")
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	data, err := n.noCache.GetDataByProjectID(ctx, projectVar)
	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	ret := make(map[string]interface{})
	ret["time"] = time.Now() // pt
	ret["notifications"] = json.RawMessage(data)
	util.WriteJson(w, ret)
}

func (n *NotificationHandler) GetAllNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	projectVar, ok := mux.Vars(r)["id"]
	if !ok {
		log.Println("no id")
		util.WriteInternalServerError(w)
		return
	}
	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	project, err := n.prRepo.GetByID(ctx, projectVar)
	if err != nil {
		if err == pgx.ErrNoRows {
			util.WriteError(w, http.StatusNotFound, "project not found.")
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
	notifications, err := n.noRepo.GetByPID(ctx, project.ID)
	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}
	util.WriteJson(w, notifications)
}

func (n *NotificationHandler) NewNotificationHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	jsonbody := r.Context().Value("json")

	projectVar, ok := mux.Vars(r)["id"]
	if !ok {
		log.Println("no id")
		util.WriteInternalServerError(w)
		return
	}

	body, ok := jsonbody.(map[string]interface{})
	if !ok {
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	title, ok := body["title"].(string)
	if !ok {
		util.WriteError(w, http.StatusBadRequest, "no title")
		return
	}
	text, ok := body["text"].(string)
	if !ok {
		util.WriteError(w, http.StatusBadRequest, "no text")
		return
	}

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	project, err := n.prRepo.GetByID(ctx, projectVar)
	if err != nil {
		if err == pgx.ErrNoRows {
			util.WriteError(w, http.StatusNotFound, "project not found.")
		} else {
			util.WriteInternalServerError(w)
		}
		return
	}

	if project.UserID != authUser.ID {
		util.WriteStatus(w, http.StatusForbidden)
		return
	}
	now := time.Now()
	no := &domain.Notification{
		PID:       project.ID,
		Status:    domain.NOTIFICATION_STATUS_ACTIVE,
		Title:     title,
		Text:      text,
		CreatedAt: now,
		ActivedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = n.noRepo.Insert(ctx, no)
	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	util.WriteJson(w, no)
}

func NewNotificationHandler(
	r *mux.Router,
	authMiddleware mux.MiddlewareFunc,
	noRepo domain.NotificationRepository,
	prRepo domain.ProjectRepository,
	noCache domain.NotificationCache,
	prefix string,
) *NotificationHandler {
	n := &NotificationHandler{
		noCache: noCache,
		noRepo:  noRepo,
		prRepo:  prRepo,
		router:  r,
	}

	n.router = r.PathPrefix(prefix).Subrouter()
	n.router.HandleFunc("/{id}/", n.GetNotificationsHandler).Methods("GET")

	authRouter := n.router.NewRoute().Subrouter()
	authRouter.Use(authMiddleware)
	authRouter.HandleFunc("/{id}/all", n.GetAllNotificationsHandler).Methods("GET")

	jsonRouter := authRouter.NewRoute().Subrouter()
	jsonRouter.Use(middleware.JsonBodyMiddleware)
	jsonRouter.HandleFunc("/{id}/new", n.NewNotificationHandler).Methods("POST")

	return n
}
