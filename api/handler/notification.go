package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/doorbash/backend-services/api/util"
	"github.com/doorbash/backend-services/api/util/middleware"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgtype"
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
	pid := mux.Vars(r)["id"]
	t := util.GetUrlQueryParam(r, "time")
	var _time *time.Time
	if t != "" {
		var err error
		parsedTime, err := time.Parse(time.RFC3339, t)
		if err != nil {
			log.Println(err)
			util.WriteError(w, http.StatusBadRequest, "bad time")
			return
		}
		_time = &parsedTime
	}
	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	data, activeTime, err := n.noCache.GetDataByProjectID(ctx, pid)
	if err != nil {
		log.Println(err)
		if err == redis.Nil {
			ctx, cancel := util.GetContextWithTimeout(r.Context())
			defer cancel()
			_activeTime, _expire, _data, err := n.noRepo.GetDataByPID(ctx, pid)
			if err != nil {
				log.Println(err)
				util.WriteStatus(w, http.StatusNotFound)
				return
			}
			data = _data
			activeTime = _activeTime
			ctx, cancel = util.GetContextWithTimeout(context.Background())
			defer cancel()
			if *_expire < 0 {
				log.Println("expire < 0, expire:", _expire, "seconds")
				util.WriteStatus(w, http.StatusNotFound)
				return
			}
			log.Println("UpdateProjectData():", "activeTime:", *activeTime, "expire:", *_expire)
			err = n.noCache.UpdateProjectData(ctx, pid, *data, *activeTime, time.Duration(*_expire)*time.Second)
			if err != nil {
				log.Println(err)
				util.WriteStatus(w, http.StatusNotFound)
				return
			}
		} else {
			util.WriteStatus(w, http.StatusNotFound)
			return
		}
	}
	log.Println("_time:", _time, "activeTime:", *activeTime)
	if _time != nil && !activeTime.Before(*_time) {
		log.Println("_time != nil && !activeTime.Before(*_time)")
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	ret := make(map[string]interface{})
	ret["time"] = activeTime
	ret["notifications"] = json.RawMessage(*data)
	util.WriteJson(w, ret)
}

func (n *NotificationHandler) GetAllNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	pid, ok := mux.Vars(r)["id"]
	if !ok {
		log.Println("no id")
		util.WriteInternalServerError(w)
		return
	}
	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	project, err := n.prRepo.GetByID(ctx, pid)
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

	when, ok := body["when"].(string)
	if !ok {
		when = "now"
	}

	var scheduleTime time.Time
	var expireTime time.Time

	switch when {
	case "later":
		st, ok := body["schedule_time"].(string)
		if !ok {
			util.WriteError(w, http.StatusBadRequest, "no schedule_time")
			return
		}
		var err error
		scheduleTime, err = time.Parse(time.RFC3339, st)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad schedule_time: %s", st))
			return
		}
		if scheduleTime.Before(time.Now().Add(5 * time.Minute)) {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("schedule_time: %s must be after %s", st, time.Now().Add(5*time.Minute).Format(time.RFC3339)))
			return
		}
		fallthrough
	case "now":
		et, ok := body["expire_time"].(string)
		if !ok {
			util.WriteError(w, http.StatusBadRequest, "no expire_time")
			return
		}
		var err error
		expireTime, err = time.Parse(time.RFC3339, et)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad expire_time: %s", et))
			return
		}
		if expireTime.Before(time.Now().Add(5 * time.Minute)) {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("expire_time: %s must be after %s", et, time.Now().Add(5*time.Minute).Format(time.RFC3339)))
			return
		}
	default:
		util.WriteError(w, http.StatusBadRequest, "bad when")
		return
	}

	if when == "later" && expireTime.Before(scheduleTime) {
		util.WriteError(w, http.StatusBadRequest, "bad expire time")
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
		PID:   project.ID,
		Title: title,
		Text:  text,
		CreateTime: pgtype.Timestamptz{
			Time:   now,
			Status: pgtype.Present,
		},
	}

	switch when {
	case "now":
		no.Status = domain.NOTIFICATION_STATUS_ACTIVE
		no.ActiveTime = pgtype.Timestamptz{
			Time:   now,
			Status: pgtype.Present,
		}
		no.ExpireTime = pgtype.Timestamptz{
			Time:   expireTime,
			Status: pgtype.Present,
		}
		no.ScheduleTime.Status = pgtype.Null
	case "later":
		no.Status = domain.NOTIFICATION_STATUS_SCHEDULED
		no.ScheduleTime = pgtype.Timestamptz{
			Time:   scheduleTime,
			Status: pgtype.Present,
		}
		no.ExpireTime = pgtype.Timestamptz{
			Time:   expireTime,
			Status: pgtype.Present,
		}
		no.ActiveTime.Status = pgtype.Null
	}

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = n.noRepo.Insert(ctx, no)
	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = n.noCache.SetProjectDataExpire(ctx, project.ID, 30*time.Second)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}

	util.WriteJson(w, no)
}

func (n *NotificationHandler) UpdateNotificationHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	jsonbody := r.Context().Value("json")

	body, ok := jsonbody.(map[string]interface{})
	if !ok {
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	_id, _ := body["id"].(float64)
	title, _ := body["title"].(string)
	text, _ := body["text"].(string)
	st, _ := body["schedule_time"].(string)
	et, _ := body["expire_time"].(string)

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	no, err := n.noRepo.GetByID(ctx, int(_id))

	if err != nil {
		util.WriteStatus(w, http.StatusNotFound)
		return
	}

	if no.Status != domain.NOTIFICATION_STATUS_ACTIVE && no.Status != domain.NOTIFICATION_STATUS_SCHEDULED {
		util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("notification status must be active or scheduled. status: %d", no.Status))
		return
	}

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	project, err := n.prRepo.GetByID(ctx, no.PID)

	if err != nil {
		util.WriteError(w, http.StatusNotFound, "no project found")
		return
	}

	if project.UserID != authUser.ID {
		util.WriteStatus(w, http.StatusForbidden)
		return
	}

	if title != "" {
		no.Title = title
	}

	if text != "" {
		no.Text = text
	}

	var scheduleTime time.Time
	if st != "" {
		if no.Status != domain.NOTIFICATION_STATUS_SCHEDULED {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("notification is not scheduled. status:%d", no.Status))
			return
		}
		var err error
		scheduleTime, err = time.Parse(time.RFC3339, st)
		if err != nil || scheduleTime.Before(time.Now().Add(5*time.Minute)) {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad schedule_time: %s", st))
			return
		}
		no.ScheduleTime.Time = scheduleTime
		no.ScheduleTime.Status = pgtype.Present
	}

	var expireTime time.Time
	if et != "" {
		var err error
		expireTime, err = time.Parse(time.RFC3339, et)
		if err != nil || expireTime.Before(time.Now().Add(5*time.Minute)) {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad expire_time: %s", et))
			return
		}
		if no.Status == domain.NOTIFICATION_STATUS_SCHEDULED && expireTime.Before(scheduleTime.Add(5*time.Minute)) {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("expire_time (%s) < schedule_time + 5min (%s)", et, scheduleTime.Add(5*time.Minute)))
			return
		}
		no.ExpireTime.Time = expireTime
		no.ExpireTime.Status = pgtype.Present
	}

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = n.noRepo.Update(ctx, no)

	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = n.noCache.SetProjectDataExpire(ctx, project.ID, 30*time.Second)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}

	util.WriteJson(w, no)
}

func (n *NotificationHandler) CancelNotificationHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	jsonbody := r.Context().Value("json")

	body, ok := jsonbody.(map[string]interface{})
	if !ok {
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	_id, _ := body["id"].(float64)

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	no, err := n.noRepo.GetByID(ctx, int(_id))

	if err != nil {
		util.WriteStatus(w, http.StatusNotFound)
		return
	}

	if no.Status != domain.NOTIFICATION_STATUS_ACTIVE && no.Status != domain.NOTIFICATION_STATUS_SCHEDULED {
		util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("notification status must be active or scheduled. status: %d", no.Status))
		return
	}

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	project, err := n.prRepo.GetByID(ctx, no.PID)

	if err != nil {
		util.WriteError(w, http.StatusNotFound, "no project found")
		return
	}

	if project.UserID != authUser.ID {
		util.WriteStatus(w, http.StatusForbidden)
		return
	}

	no.Status = domain.NOTIFICATION_STATUS_CANCELED

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = n.noRepo.Update(ctx, no)

	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = n.noCache.SetProjectDataExpire(ctx, project.ID, 30*time.Second)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
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
) *NotificationHandler {
	n := &NotificationHandler{
		noCache: noCache,
		noRepo:  noRepo,
		prRepo:  prRepo,
		router:  r,
	}

	n.router.HandleFunc("/{id}/notifications", n.GetNotificationsHandler).Methods("GET")

	authRouter := n.router.NewRoute().Subrouter()
	authRouter.Use(authMiddleware)
	authRouter.HandleFunc("/{id}/notifications/all", n.GetAllNotificationsHandler).Methods("GET")

	jsonRouter := authRouter.NewRoute().Subrouter()
	jsonRouter.Use(middleware.JsonBodyMiddleware)
	jsonRouter.HandleFunc("/{id}/notifications/new", n.NewNotificationHandler).Methods("POST")
	jsonRouter.HandleFunc("/notifications/update", n.UpdateNotificationHandler).Methods("POST")
	jsonRouter.HandleFunc("/notifications/cancel", n.CancelNotificationHandler).Methods("POST")

	return n
}
