package util

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
)

type Result struct {
	Ok     bool         `json:"ok"`
	Err    *string      `json:"error,omitempty"`
	Result *interface{} `json:"result,omitempty"`
}

type ResultRaw struct {
	Ok     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
}

func WriteOK(w http.ResponseWriter) {
	result := &Result{
		Ok: true,
	}
	data, err := json.Marshal(result)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func WriteError(w http.ResponseWriter, statusCode int, errorMessage string) {
	result := &Result{
		Ok:  false,
		Err: &errorMessage,
	}
	data, err := json.Marshal(result)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(data)
}

func WriteJson(w http.ResponseWriter, res interface{}) {
	result := &Result{
		Ok:     true,
		Result: &res,
	}
	data, err := json.Marshal(result)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func WriteJsonRaw(w http.ResponseWriter, res string) {
	result := &ResultRaw{
		Ok:     true,
		Result: json.RawMessage(res),
	}
	data, err := json.Marshal(result)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func WriteStatus(w http.ResponseWriter, statusCode int) {
	WriteError(w, statusCode, http.StatusText(statusCode))
}

func WriteUnauthorized(w http.ResponseWriter) {
	WriteStatus(w, http.StatusUnauthorized)
}

func WriteInternalServerError(w http.ResponseWriter) {
	WriteStatus(w, http.StatusInternalServerError)
}

func GetUrlQueryParam(r *http.Request, key string) string {
	keys, ok := r.URL.Query()[key]

	if !ok || len(keys[0]) < 1 {
		return ""
	}

	return keys[0]
}

func LoggerMiddleware(h http.Handler) http.Handler {
	return handlers.CustomLoggingHandler(os.Stdout, h, func(writer io.Writer, params handlers.LogFormatterParams) {
		log.Println("HttpLogger:", params.Request.Method, params.Request.URL.Path, params.StatusCode)
	})
}

func JsonBodyMiddleware(h func(w http.ResponseWriter, r *http.Request, body *interface{})) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			WriteInternalServerError(w)
			return
		}
		log.Println(string(data))
		var jsonBody interface{}
		err = json.Unmarshal(data, &jsonBody)
		if err != nil {
			log.Println(err)
			WriteStatus(w, http.StatusBadRequest)
			return
		}
		h(w, r, &jsonBody)
	})
}
