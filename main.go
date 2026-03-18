package main

import _ "github.com/lib/pq" //lane hates this

import (
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"encoding/json"
	"time"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"github.com/CromartyForth/chirpy/internal/profane"
)

type user struct {
	ID uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
	Email string `json:"email"`

}

func main() {
	// get env contents
	godotenv.Load()

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
	mux.HandleFunc("POST /api/validate_chirp", validate)
	mux.HandleFunc("POST /api/users", createUser)
	// POST request at /api/validate_chirp


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

func validate(w http.ResponseWriter, r *http.Request) {

	// read from body - expecting json format
	type parameters struct {
  		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("Something went wromg: %s", err))

	} else if len(params.Body) > 140 {
		fmt.Printf("Chirp Length: %v", len(params.Body))
		respondWithError(w, 400, fmt.Sprintln("Chirp is too long"))

	} else {
		fmt.Printf("Chirp Length: %v", len(params.Body))
		type respValid struct {
			Cleaned_body string `json:"cleaned_body"`
		}

		cleanBody := profane.RemoveProfane(params.Body)

		payload := respValid {
			Cleaned_body: cleanBody,
		}
		respondWithJSON(w, 200, payload)
	}
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type respErr struct {
		Error string `json:"error"`
	}

	payload := respErr {
		Error: msg,
	}

	respondWithJSON(w, code, payload)
	return
}

func respondWithJSON(w http.ResponseWriter, code int, payload any){
	// payload is a struct for json marshalling

	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
	return
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


