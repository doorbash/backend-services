package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/doorbash/backend-services/api/domain"
	"github.com/doorbash/backend-services/api/util"
	"github.com/doorbash/backend-services/api/util/middleware"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type NotificationHandler struct {
	noCache domain.NotificationCache
	noRepo  domain.NotificationRepository
	prRepo  domain.ProjectRepository
	router  *mux.Router
}

func (n *NotificationHandler) GetNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	pid := mux.Vars(r)["id"]
	t := r.URL.Query().Get("time")
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
	activeTime, err := n.noCache.GetTimeByProjectID(ctx, pid)
	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	if _time != nil && !_time.Before(*activeTime) {
		util.WriteStatus(w, http.StatusNotFound)
		return
	}
	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	data, err := n.noCache.GetDataByProjectID(ctx, pid)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}
	log.Println(*data)
	ret := map[string]interface{}{
		"time":          time.Now().Format(time.RFC3339),
		"notifications": json.RawMessage(*data),
	}
	util.WriteJson(w, ret)
}

func (n *NotificationHandler) NotificationClickedHandler(w http.ResponseWriter, r *http.Request) {
	pid := mux.Vars(r)["id"]
	ids := r.URL.Query().Get("ids")
	idArr := strings.Split(ids, ",")
	if len(idArr) > 10 {
		util.WriteError(w, http.StatusBadRequest, "too much ids. max length is 10")
		return
	}
	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	err := n.noCache.IncrClicksIds(ctx, pid, idArr)
	if err != nil {
		log.Println(err)
		util.WriteInternalServerError(w)
		return
	}
	util.WriteOK(w)
}

func (n *NotificationHandler) GetAllNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	pid, ok := mux.Vars(r)["id"]
	if !ok {
		log.Println("no id")
		util.WriteInternalServerError(w)
		return
	}
	query := r.URL.Query()
	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit <= 0 {
		util.WriteError(w, http.StatusBadRequest, "bad limit")
		return
	}
	offset, err := strconv.Atoi(query.Get("offset"))
	if err != nil || offset < 0 {
		util.WriteError(w, http.StatusBadRequest, "bad offset")
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
	notifications, err := n.noRepo.GetByPID(ctx, project.ID, limit, offset)
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

	pid, ok := mux.Vars(r)["id"]
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

	when, _ := body["when"].(string)
	if when == "" {
		when = "now"
	}

	var scheduleTime time.Time
	var expireTime time.Time

	switch when {
	case "later":
		st, _ := body["schedule_time"].(string)
		if st == "" {
			util.WriteError(w, http.StatusBadRequest, "no schedule_time")
			return
		}
		var err error
		scheduleTime, err = time.Parse(time.RFC3339, st)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad schedule_time: %s", st))
			return
		}
		if scheduleTime.Before(time.Now().Add(15 * time.Minute)) {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("schedule_time: %s must be after %s", st, time.Now().Add(15*time.Minute).Format(time.RFC3339)))
			return
		}
		fallthrough
	case "now":
		et, _ := body["expire_time"].(string)
		if et == "" {
			util.WriteError(w, http.StatusBadRequest, "no expire_time")
			return
		}
		var err error
		expireTime, err = time.Parse(time.RFC3339, et)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad expire_time: %s", et))
			return
		}
		if expireTime.Before(time.Now().Add(30 * time.Minute)) {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("expire_time: %s must be after %s", et, time.Now().Add(30*time.Minute).Format(time.RFC3339)))
			return
		}
	default:
		util.WriteError(w, http.StatusBadRequest, "bad when")
		return
	}

	if when == "later" && expireTime.Before(scheduleTime.Add(30*time.Minute)) {
		util.WriteError(w, http.StatusBadRequest, "bad expire time")
		return
	}

	title, _ := body["title"].(string)
	if title == "" {
		util.WriteError(w, http.StatusBadRequest, "no title")
		return
	}
	text, _ := body["text"].(string)
	if text == "" {
		util.WriteError(w, http.StatusBadRequest, "no text")
		return
	}

	image, _ := body["image"].(string)
	if image != "" && !strings.HasPrefix(image, "http") {
		log.Println("bad image:", image)
		util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad image:%s", image))
		return
	}

	action, _ := body["action"].(string)
	var extra string
	switch action {
	case "":
	case "activity":
		name, _ := body["name"].(string)
		parent, _ := body["parent"].(string)
		if parent == "" {
			extra = name
		} else {
			extra = fmt.Sprintf("%s %s", parent, name)
		}
	case "link":
		extra, _ = body["url"].(string)
	case "update":
		url, _ := body["url"].(string)
		version, _ := body["version"].(float64)
		extra = fmt.Sprintf("%s %d", url, int(version))
	default:
		log.Println("bad action:", action)
		util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad action %s", action))
		return
	}

	priority, _ := body["priority"].(string)
	switch priority {
	case "default", "low", "high", "min", "max":
	case "":
		priority = "default"
	default:
		log.Println("bad priority:", priority)
		util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad priority %s", priority))
		return
	}

	var bigText string
	var bigImage string

	style, _ := body["style"].(string)
	switch style {
	case "normal", "big":
	case "":
		style = "normal"
	case "big-text":
		bigText, _ = body["big-text"].(string)
		if bigText == "" {
			log.Println("no big-text")
			util.WriteError(w, http.StatusBadRequest, "no big-text")
			return
		}
	case "big-image":
		bigImage, _ = body["big-image"].(string)
		if bigImage == "" {
			log.Println("no big-image")
			util.WriteError(w, http.StatusBadRequest, "no big-image")
			return
		}
	default:
		log.Println("bad style:", style)
		util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad style %s", style))
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

	now := time.Now()
	no := &domain.Notification{
		PID:        project.ID,
		Title:      title,
		Text:       text,
		CreateTime: &now,
		Priority:   priority,
		Style:      style,
	}

	if bigText != "" {
		no.BigText = &bigText
	}

	if bigImage != "" {
		no.BigImage = &bigImage
	}

	if image != "" {
		no.Image = &image
	}

	if action != "" {
		no.Action = &action
		no.Extra = &extra
	}

	switch when {
	case "now":
		no.Status = domain.NOTIFICATION_STATUS_ACTIVE
		no.ActiveTime = &now
		no.ExpireTime = &expireTime
	case "later":
		no.Status = domain.NOTIFICATION_STATUS_SCHEDULED
		no.ScheduleTime = &scheduleTime
		no.ExpireTime = &expireTime
	}

	topic, ok := mux.Vars(r)["topic"]
	if !ok || topic == "" {
		ctx, cancel = util.GetContextWithTimeout(r.Context())
		defer cancel()
		err = n.noRepo.Insert(ctx, no)
		if err != nil {
			log.Println(err)
			util.WriteStatus(w, http.StatusBadRequest)
			return
		}
	} else {
		rand.Seed(time.Now().UnixNano())
		no.ID = rand.Intn(10000000) + 1
		b, err := json.Marshal(no)
		if err != nil {
			log.Println(err)
			util.WriteStatus(w, http.StatusBadRequest)
			return
		}
		ctx, cancel = util.GetContextWithThisTimeout(r.Context(), 20*time.Second)
		defer cancel()
		err = util.SendNotification(ctx, project.ID, topic, map[string]string{
			"type": "notification",
			"data": string(b),
		})
		if err != nil {
			log.Println(err)
			util.WriteStatus(w, http.StatusBadRequest)
			return
		}
	}

	util.WriteJson(w, no)
}

func (n *NotificationHandler) UpdateNotificationHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	jsonBody := r.Context().Value("json")

	body, ok := jsonBody.(map[string]interface{})
	if !ok {
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	id, ok := body["id"].(float64)
	if !ok {
		util.WriteError(w, http.StatusBadRequest, "no id")
		return
	}
	title, _ := body["title"].(string)
	text, _ := body["text"].(string)
	st, _ := body["schedule_time"].(string)
	et, _ := body["expire_time"].(string)
	action, _ := body["action"].(string)
	bigText, _ := body["big-text"].(string)
	bigImage, _ := body["big-image"].(string)

	var extra string
	switch action {
	case "":
	case "activity":
		name, _ := body["name"].(string)
		parent, _ := body["parent"].(string)
		if parent == "" {
			extra = name
		} else {
			extra = fmt.Sprintf("%s %s", parent, name)
		}
	case "link":
		extra, _ = body["url"].(string)
	case "update":
		url, _ := body["url"].(string)
		version, _ := body["version"].(string)
		extra = fmt.Sprintf("%s%s", url, version)
	default:
		log.Println("bad action:", action)
		util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad action %s", action))
		return
	}

	priority, _ := body["priority"].(string)
	switch priority {
	case "", "default", "low", "high", "min", "max":
	default:
		log.Println("bad priority:", priority)
		util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad priority %s", priority))
		return
	}

	style, _ := body["style"].(string)
	switch style {
	case "", "normal", "big", "big-text", "big-image":
	default:
		log.Println("bad style:", style)
		util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad style %s", style))
		return
	}

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	no, err := n.noRepo.GetByID(ctx, int(id))

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

	if bigText != "" {
		if no.Style != "big-text" {
			util.WriteError(w, http.StatusBadRequest, "notification style is not big-text")
			return
		}
		no.BigText = &bigText
	}

	if bigImage != "" {
		if no.Style != "big-image" {
			util.WriteError(w, http.StatusBadRequest, "notification style is not big-image")
			return
		}
		no.BigImage = &bigImage
	}

	if action != "" {
		no.Action = &action
		no.Extra = &extra
	}

	if priority != "" {
		no.Priority = priority
	}

	if style != "" {
		no.Style = style
	}

	var scheduleTime time.Time
	if st != "" {
		if no.Status != domain.NOTIFICATION_STATUS_SCHEDULED {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("notification is not scheduled. status:%d", no.Status))
			return
		}
		var err error
		scheduleTime, err = time.Parse(time.RFC3339, st)
		if err != nil || scheduleTime.Before(time.Now().Add(15*time.Minute)) {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad schedule_time: %s", st))
			return
		}
		no.ScheduleTime = &scheduleTime
	}

	var expireTime time.Time
	if et != "" {
		var err error
		expireTime, err = time.Parse(time.RFC3339, et)
		if err != nil || expireTime.Before(time.Now().Add(30*time.Minute)) {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad expire_time: %s", et))
			return
		}
		if no.Status == domain.NOTIFICATION_STATUS_SCHEDULED && expireTime.Before(scheduleTime.Add(30*time.Minute)) {
			util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("expire_time (%s) < schedule_time + 30min (%s)", et, scheduleTime.Add(30*time.Minute)))
			return
		}
		no.ExpireTime = &expireTime
	}

	image, _ := body["image"].(string)
	if image != "" && !strings.HasPrefix(image, "http") {
		log.Println("bad image url:", image)
		util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("bad image url: %s", image))
		return
	}

	if image != "" {
		no.Image = &image
	}

	ctx, cancel = util.GetContextWithTimeout(r.Context())
	defer cancel()
	err = n.noRepo.Update(ctx, no)

	if err != nil {
		log.Println(err)
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	util.WriteJson(w, no)
}

func (n *NotificationHandler) CancelNotificationHandler(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value("user").(middleware.AuthUserValue)
	jsonBody := r.Context().Value("json")

	body, ok := jsonBody.(map[string]interface{})
	if !ok {
		util.WriteStatus(w, http.StatusBadRequest)
		return
	}

	id, ok := body["id"].(float64)
	if !ok {
		util.WriteError(w, http.StatusBadRequest, "no id")
		return
	}

	ctx, cancel := util.GetContextWithTimeout(r.Context())
	defer cancel()
	no, err := n.noRepo.GetByID(ctx, int(id))

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
	n.router.HandleFunc("/{id}/notifications/clicked", n.NotificationClickedHandler).Methods("GET")

	authRouter := n.router.NewRoute().Subrouter()
	authRouter.Use(authMiddleware)
	authRouter.HandleFunc("/{id}/notifications/all", n.GetAllNotificationsHandler).Methods("GET")

	jsonRouter := authRouter.NewRoute().Subrouter()
	jsonRouter.Use(middleware.JsonBodyMiddleware)
	jsonRouter.HandleFunc("/{id}/notifications/new", n.NewNotificationHandler).Methods("POST")
	jsonRouter.HandleFunc("/{id}/notifications/new/fcm/{topic}", n.NewNotificationHandler).Methods("POST")
	jsonRouter.HandleFunc("/notifications/update", n.UpdateNotificationHandler).Methods("POST")
	jsonRouter.HandleFunc("/notifications/cancel", n.CancelNotificationHandler).Methods("POST")

	return n
}
