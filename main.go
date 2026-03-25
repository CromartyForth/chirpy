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
	"errors"
	"sync/atomic"
	"github.com/CromartyForth/chirpy/internal/profane"
	"database/sql"
	"github.com/CromartyForth/chirpy/internal/database"
	"github.com/CromartyForth/chirpy/internal/auth"

)

type MyUser struct {
	ID uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email string `json:"email"`
	IsChirpyRed bool `json:"is_chirpy_red"`
}

type response struct {
	MyUser
	Token string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

type refreshJWT struct {
	Token string `json:"token"`
}

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries *database.Queries
	platform string
	aSecret string
}

type login struct {
	Email string `json:"email"`
	Password string `json:"password"`
}

type myChirp struct {
	ID uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body string `json:"body"`
	UserID uuid.UUID `json:"user_id"`
}

type chirpParams struct {
	Body string `json:"body"`
}

type polkaEvent struct {
  	Event string `json:"event"`
  	Data struct {
    	UserID uuid.UUID `json:"user_id"`
  	} `json:"data"`
}

func main() {
	// initialise apiConfig
	cfig := apiConfig{}

	// get env contents store in cfig
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	cfig.platform = os.Getenv("PLATFORM")
	cfig.aSecret = os.Getenv("SECRET")

	// open connection to database, store in cfig
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("error connecting to database")
		os.Exit(1)
	}
	cfig.dbQueries = database.New(db)
	

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
	mux.HandleFunc("GET /api/chirps/{chirpID}",cfig.getChirpsByID)
	mux.HandleFunc("POST /api/login", cfig.login)
	mux.HandleFunc("POST /api/refresh", cfig.refresh)
	mux.HandleFunc("POST /api/revoke", cfig.revoke)
	mux.HandleFunc("PUT /api/users", cfig.updateUser)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", cfig.deleteChirpByID)
	mux.HandleFunc("POST /api/polka/webhooks", cfig.upgradeUserByID)
	
	
	// start the server
	err = server.ListenAndServe()
	if err != nil {
		fmt.Printf("Server Error: %v", err)
		os.Exit(1)
	}
}

func (a *apiConfig) upgradeUserByID (w http.ResponseWriter, r *http.Request) {
	// get the params from the body
	params := polkaEvent{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error decoding body")
	}

	// is upgrade event?
	if params.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// update user to chirpy red.
	_, err = a.dbQueries.UpgradeUserByID(r.Context(), params.Data.UserID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	// respond ok
	w.WriteHeader(http.StatusNoContent)
}


func (a *apiConfig) deleteChirpByID (w http.ResponseWriter, r *http.Request) {
	// get the auth token from the header
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error extracting token")
		return
	}

	// validate token
	user, err := auth.ValidateJWT(token, a.aSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid token")
	}

	// get the path "id" value, http.Request.PathValue
	chirpID := r.PathValue("chirpID")
	// convert to UUID
	chirpUUID, err := uuid.Parse(chirpID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid UUID")
		return
	}

	// get chirp
	chirp, err := a.dbQueries.GetChirpByID(r.Context(), chirpUUID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Chirp not found")
		return
	}

	// is user owner of chirp
	if user != chirp.UserID {
		respondWithError(w, http.StatusForbidden, "ids non matching")
		return
	}

	// delete chirp by id
	err = a.dbQueries.DeleteChirpByID(r.Context(), chirp.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error deleting chirp")
	}

	// respond// respond with http.ResonseWriter
	w.WriteHeader(http.StatusNoContent)
}


func (a *apiConfig) updateUser(w http.ResponseWriter, r *http.Request) {
	// get the auth token from the header
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error extracting token")
		return
	}

	// verify token
	user, err := auth.ValidateJWT(token, a.aSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Error validating token: %v", err))
		return
	}
	
	// get params from body
	params := login{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("Error decoding POST body: %v", err))
		return
	}

	// hash new password
	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating password hash")
	}

	// prepare update params
	updateParams := database.UpdateUserParams{
		Email: params.Email,
		HashedPassword: hashedPassword,
		ID: user,
	}

	// update password and email
	updatedUser, err := a.dbQueries.UpdateUser(r.Context(), updateParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating database")
	}

	// prepare update.
	response := MyUser{ 
		ID: user,  
    	CreatedAt: updatedUser.CreatedAt,
    	UpdatedAt: updatedUser.UpdatedAt,
    	Email: updatedUser.Email,
		IsChirpyRed: updatedUser.IsChirpyRed,
	}

	respondWithJSON(w, http.StatusOK, response)

}


func (a *apiConfig) revoke(w http.ResponseWriter, r *http.Request) {
	// get the refresh token from the header
	refreshToken, err:= auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error extracting token")
		return
	}

	a.dbQueries.RevokeRefreshToken(r.Context(), refreshToken)

	// respond with http.ResonseWriter
	w.WriteHeader(http.StatusNoContent)
	
}


func (a *apiConfig) refresh(w http.ResponseWriter, r *http.Request) {
	// get the refresh token from the header
	refreshToken, err:= auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error extracting token")
		return
	}

	// get user from refresh token
	user, err := a.dbQueries.GetUserFromRefreshToken(r.Context(), refreshToken)

	if errors.Is(err, sql.ErrNoRows) {
		// token not found, expired, or revoked
		respondWithError(w, http.StatusUnauthorized, "invalid token")
		return
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error querying database")
		return
	}

	// get new JWT token and package up for response
	newToken, err := auth.MakeJWT(user.UserID, a.aSecret)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating new token")
	}
	respJson := refreshJWT{
		Token: newToken,
	}

	// respond with token.
	respondWithJSON(w, http.StatusOK, respJson)
}


func (a *apiConfig) login(w http.ResponseWriter, r *http.Request) {
	// get the params from the json body
	params := login{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("Error decoding POST body: %v", err))
		return
	}

	// get user by email
	user, err := a.dbQueries.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	// check hashes match
	isMatch, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil || isMatch == false{
		respondWithError(w, 401, "Unauthorised")
		return
	}

	// make JWT token
	myToken, err := auth.MakeJWT(user.ID, a.aSecret)
	if err != nil {
		respondWithError(w, 500, "Error making token")
	}

	// get refresh token
	rfToken := auth.MakeRefreshToken()

	// register refresh token to user
	rfParams := database.CreateRefreshTokenParams{
		ID: rfToken,
		UserID: user.ID,
	}

	rfID, err := a.dbQueries.CreateRefreshToken(r.Context(), rfParams)
	if err != nil {
		respondWithError(w, 500, "Error creating RF Token")
	}

	// format response
	myUser := MyUser{
		ID: user.ID,
		CreatedAt: user.CreatedAt, 
		UpdatedAt: user.CreatedAt,
		Email: user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}
	myResponse := response{
		MyUser: myUser,
		Token: myToken,
		RefreshToken: rfID,
	}

	// respond to login
	respondWithJSON(w, 200, myResponse)
}


func (a *apiConfig) getChirpsByID(w http.ResponseWriter, r *http.Request) {
	// get the path "id" value, http.Request.PathValue
	chirpID := r.PathValue("chirpID")
	// convert to UUID
	chirpUUID, err := uuid.Parse(chirpID)
	if err != nil {
		respondWithError(w, 404, "Invalid UUID")
		return
	}

	// git it from the database
	chirp, err := a.dbQueries.GetChirpByID(r.Context(), chirpUUID)
	if err != nil {
		respondWithError(w, 404, "Chirp not found")
		return
	}
	
	// format response
	chirpJSON := myChirp{
		ID: chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body: chirp.Body,
		UserID: chirp.UserID,
	}

	// sent response
	respondWithJSON(w, 200, chirpJSON)
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
	params := chirpParams{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, "Error decoding POST body")
		return
	}

	// Validate params
	if params.Body == ""{
		respondWithError(w, 400, "missing chirp parameters")
		return
	}

	// Validate JWT
	token, err:= auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 500, "Error extracting token")
		return
	}
	validID, err := auth.ValidateJWT(token, a.aSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Error validating token: %v", err))
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
		UserID: validID,
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
	params := login{}
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
	if params.Password == "" {
		respondWithError(w, 400, "Password not supplied")
		return
	}

	// get hash of password
	hash, err := auth.HashPassword(params.Password)
	
	// CreateUserParams
	userParams := database.CreateUserParams {
		Email: params.Email,
		HashedPassword: hash,
	}

	// create user
	user, err := a.dbQueries.CreateUser(r.Context(), userParams)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("Error creating user: %v", err))
		return
	}

	// map database respose to user struct
	response := MyUser{
		ID: user.ID,
		CreatedAt: user.CreatedAt, 
		UpdatedAt: user.CreatedAt,
		Email: user.Email,
		IsChirpyRed: user.IsChirpyRed,
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


