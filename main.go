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
	"database/sql"
	"github.com/CromartyForth/chirpy/internal/database"

)

type myUser struct {
	ID uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email string `json:"email"`
}

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries *database.Queries
	platform string
}

type email struct {
	Email string `json:"email"`
}

type myChirp struct {
	ID uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body string `json:"body"`
	UserID uuid.UUID `json:"user_id"`
}


func main() {
	// initialise apiConfig
	cfig := apiConfig{}

	// get env contents
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")

	// open connection to database, store in cfig
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("error connecting to database")
		os.Exit(1)
	}
	// add to cfig
	cfig.dbQueries = database.New(db)
	cfig.platform = platform

	// Create a new http.ServeMux
	mux := http.NewServeMux()

	// Create a new http.Server struct.
	server := http.Server{
		Addr: ":8080",
		Handler: mux,
	}
	
	fileServer :=  http.FileServer(http.Dir("."))

	mux.Handle("/app/", http.StripPrefix("/app", cfig.middlewareMetricInc(fileServer)))
	mux.HandleFunc("GET /api/healthz", Readyness)
	mux.HandleFunc("GET /admin/metrics", cfig.getCount)
	mux.HandleFunc("POST /admin/reset", cfig.reset)
	// mux.HandleFunc("POST /api/validate_chirp", validate)
	mux.HandleFunc("POST /api/users", cfig.createUser)
	mux.HandleFunc("POST /api/chirps", cfig.createChirp)
	mux.HandleFunc("GET /api/chirps", cfig.getChirps)
	
	// start the server
	err = server.ListenAndServe()
	if err != nil {
		fmt.Printf("Server Error: %v", err)
		os.Exit(1)
	}
}

func (a *apiConfig) getChirps(w http.ResponseWriter, r *http.Request) {
	// collect them all
	chirpArray, err := a.dbQueries.GetAllChirps(r.Context())
	if err != nil {
		respondWithError(w, 500, "error recieving from database")
	}
	
	// create array of chirps
	newChirpArray := make([]myChirp, 0)
	// itterate over chirpArray converting to our format
	for _, chirp := range(chirpArray) {
		newChirp := myChirp{
		ID: chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body: chirp.Body,
		UserID: chirp.UserID,
	}
		newChirpArray = append(newChirpArray, newChirp)
	}

	respondWithJSON(w, 200, newChirpArray)
}

func (a *apiConfig) createChirp(w http.ResponseWriter, r *http.Request) {
	
	// get the params from the json body
	params := myChirp{} // partially used
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, "Error decoding POST body")
		return
	}

	// Validate params
	if params.Body == "" || params.UserID == uuid.Nil {
		respondWithError(w, 400, "missing chirp or username")
		return
	}

	// check length - 140 should be in config
	if len(params.Body) > 140 {
		fmt.Printf("Chirp Length: %v", len(params.Body))
		respondWithError(w, 400, "Chirp is too long")
	}

	// remove profanity
	cleanBody := profane.RemoveProfane(params.Body)

	// chirp to chirp
	chirpChirp := database.CreatChirpParams {
		Body: cleanBody,
		UserID: params.UserID,
	}

	chirp, err := a.dbQueries.CreatChirp(r.Context(),chirpChirp)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("Error creating chirp in database: %v", err))
		return
	}

	// chirp to chirp
	newChirp := myChirp{
		ID: chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body: chirp.Body,
		UserID: chirp.UserID,
	}

	// respond ok
	respondWithJSON(w, 201, newChirp)
}


func (a *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	
	// get the email string from json body.
	params := email{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, "Error decoding POST body")
		return
	}

	// validate email - This could be a whole fuction using regex ect
	if params.Email == "" {
		respondWithError(w, 400, "Email not supplied")
		return
	}

	// create user
	user, err := a.dbQueries.CreateUser(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("Error creating user: %v", err))
		return
	}

	// map database respose to user struct
	response := myUser{
		ID: user.ID,
		CreatedAt: user.CreatedAt, 
		UpdatedAt: user.CreatedAt,
		Email: user.Email,
	}

	//marshal and respond
	respondWithJSON(w, 201, response)
}



func Readyness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte("OK\n"))
	if err != nil {
		fmt.Printf("Error writing body: %v", err)
	}
}

/* To be removed.
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
		respondWithError(w, 400, "Chirp is too long")

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
*/

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type respErr struct {
		Error string `json:"error"`
	}

	payload := respErr {
		Error: msg,
	}

	respondWithJSON(w, code, payload)
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
}


// Metrics Middleware

func (a *apiConfig) middlewareMetricInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		newCount := a.fileserverHits.Add(1)
		fmt.Printf("New Count is: %v\n", newCount)
		next.ServeHTTP(w, r)
	}) 
}


func (a *apiConfig) getCount(w http.ResponseWriter, r *http.Request) {
	
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

func (a *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	// development only
	if a.platform != "dev" {
		respondWithError(w, 403, "Forbidden!")
		return
	}

	// delete all users
	err := a.dbQueries.DeleteAllUsers(r.Context())
	if err != nil {
		respondWithError(w, 500, "error deleting users")
		return
	}
	
	// reset count and return old value
	count := a.fileserverHits.Swap(0)
	resetTxt := fmt.Sprintf("Count of %v reset to 0\n", count)

	// write response
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err = w.Write([]byte(string(resetTxt)))
	if err != nil {
		fmt.Printf("Error writing body: %v\n", err)
	}
}
// End Metrics Middleware


