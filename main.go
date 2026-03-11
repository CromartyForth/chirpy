package main

import (
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
)


func main() {
	// create metrics middleware storage
	metrics := apiMetrics{}

	// Create a new http.ServeMux
	mux := http.NewServeMux()

	// Create a new http.Server struct.
	server := http.Server{
		Addr: ":8080",
		Handler: mux,
	}
	
	fileServer :=  http.FileServer(http.Dir("."))

	mux.Handle("/app/", http.StripPrefix("/app", metrics.middlewareMetricInc(fileServer)))
	mux.HandleFunc("GET /api/healthz", Readyness)
	mux.HandleFunc("GET /admin/metrics", metrics.getCount)
	mux.HandleFunc("POST /admin/reset", metrics.reset)


	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("Server Error: %v", err)
		os.Exit(1)
	}


}

func Readyness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte("OK\n"))
	if err != nil {
		fmt.Printf("Error writing body: %v", err)
	}
}

// Metrics Middleware
type apiMetrics struct {
	fileserverHits atomic.Int32
}

func (a *apiMetrics) middlewareMetricInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		newCount := a.fileserverHits.Add(1)
		fmt.Printf("New Count is: %v\n", newCount)
		next.ServeHTTP(w, r)
	}) 
}


func (a *apiMetrics) getCount(w http.ResponseWriter, r *http.Request) {
	
	// get count
	count := a.fileserverHits.Load()
	
	// html
	metricsHtml := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", count)
	
	// write response
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(200)
	_, err := w.Write([]byte(string(metricsHtml)))
	if err != nil {
		fmt.Printf("Error writing body: %v\n", err)
	}
}

func (a *apiMetrics) reset(w http.ResponseWriter, r *http.Request) {
	// reset count and return old value
	count := a.fileserverHits.Swap(0)
	resetTxt := fmt.Sprintf("Count of %v reset to 0\n", count)

	// write response
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte(string(resetTxt)))
	if err != nil {
		fmt.Printf("Error writing body: %v\n", err)
	}
}
// End Metrics Middleware


