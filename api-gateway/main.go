package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/casbin/casbin"
	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
)

type ApiError struct {
	Error string
}

func main() {
	router := mux.NewRouter()

	authEnforcer, err := casbin.NewEnforcerSafe("./auth_model.conf", "./policy.csv")
	if err != nil {
		log.Fatal(err)
	}

	//USER MICROSERVICE
	router.HandleFunc("/login", redirect("http://localhost:3001"))
	router.HandleFunc("/user", redirect("http://localhost:3001"))
	router.HandleFunc("/user/deleated", authMiddleware(redirect("http://localhost:3001")))
	router.HandleFunc("/user/{id}", authMiddleware(redirect("http://localhost:3001")))
	router.HandleFunc("/user/delete/{id}", authMiddleware(redirect("http://localhost:3001")))
	router.HandleFunc("/user/{userId}/{workoutId}", authMiddleware(redirect("http://localhost:3001")))
	router.HandleFunc("/userWorkoutRefs/{id}", authMiddleware(redirect("http://localhost:3001")))
	router.HandleFunc("/user/restore/{id}", authMiddleware(redirect("http://localhost:3001")))

	//WORKOUT MICROSERVICE
	router.HandleFunc("/exercise", authMiddleware(redirect("http://localhost:3002")))
	router.HandleFunc("/userWorkouts", authMiddleware(redirect("http://localhost:3002")))
	router.HandleFunc("/workout", authMiddleware(redirect("http://localhost:3002")))
	router.HandleFunc("/workout/rate/{id}/{rating}", redirect("http://localhost:3002"))
	router.HandleFunc("/workout/{id}", authMiddleware(redirect("http://localhost:3002")))
	router.HandleFunc("/workout/calendar/{userId}/{workoutId}/{date}/{workoutName}", authMiddleware(redirect("http://localhost:3002")))
	router.HandleFunc("/workout/calendar/{id}", authMiddleware(redirect("http://localhost:3002")))
	router.HandleFunc("/exercise/{coachId}", authMiddleware(redirect("http://localhost:3002")))

	//COMMENT MICROSERVICE
	router.HandleFunc("/comment", authMiddleware(redirect("http://localhost:3003")))
	router.HandleFunc("/comment/{userId}/{workoutId}/{commentType}", authMiddleware(redirect("http://localhost:3003")))
	router.HandleFunc("/comment/{workoutId}/{commentType}", authMiddleware(redirect("http://localhost:3003")))

	//RATING MICROSERVICE
	router.HandleFunc("/rating", authMiddleware(redirect("http://localhost:3004")))
	router.HandleFunc("/rating/{id}", authMiddleware(redirect("http://localhost:3004")))
	router.HandleFunc("/rating/{subjectId}", authMiddleware(redirect("http://localhost:3004")))
	router.HandleFunc("/rating/{userId}/{workoutId}/{ratingType}", authMiddleware(redirect("http://localhost:3004")))

	//COMPLAINT SERVICE
	router.HandleFunc("/complaint", authMiddleware(redirect("http://localhost:4001")))
	router.HandleFunc("/complaint/{id}", authMiddleware(redirect("http://localhost:4001")))
	router.HandleFunc("/complaint/user/{id}", authMiddleware(redirect("http://localhost:4001")))
	router.HandleFunc("/complaint/subject/{id}", authMiddleware(redirect("http://localhost:4001")))

	//BLOCK SERVICE
	router.HandleFunc("/block", authMiddleware(redirect("http://localhost:4002")))
	router.HandleFunc("/block/{id}", authMiddleware(redirect("http://localhost:4002")))
	router.HandleFunc("/block/user/{id}", authMiddleware(redirect("http://localhost:4002")))
	router.HandleFunc("/block/subject/{id}", authMiddleware(redirect("http://localhost:4002")))

	http.ListenAndServe(":3000", Authorizer(authEnforcer)(router))
}

func authMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("x-jwt-token")
		token, err := validateJWT(tokenStr)

		if err != nil {
			WriteJSON(w, http.StatusForbidden, ApiError{Error: "invalid token"})
			return
		}

		claims := token.Claims.(jwt.MapClaims)

		username := claims["username"]
		password := claims["password"]

		result := validateUser(username.(string), password.(string))

		if result == "" {
			WriteJSON(w, http.StatusForbidden, ApiError{Error: "invalid token"})
			return
		}
		h.ServeHTTP(w, r)
	})
}

func Authorizer(e *casbin.Enforcer) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {

			role := ""
			tokenStr := r.Header.Get("x-jwt-token")
			token, err := validateJWT(tokenStr)

			if err != nil && tokenStr != "" {
				WriteJSON(w, http.StatusForbidden, ApiError{Error: "invalid token"})
				return
			}

			if tokenStr != "" {
				claims := token.Claims.(jwt.MapClaims)
				role = claims["role"].(string)
			}

			if role == "" {
				role = "anonymous"
			}

			// casbin rule enforcing
			res, err := e.EnforceSafe(role, r.URL.Path, r.Method)

			if err != nil {
				WriteJSON(w, http.StatusForbidden, ApiError{Error: "Authorization error"})
				return
			}
			if res {
				next.ServeHTTP(w, r)
			} else {
				WriteJSON(w, http.StatusForbidden, ApiError{Error: "Authorization error"})
				return
			}
		}

		return http.HandlerFunc(fn)
	}
}

func validateUser(username, password string) string {
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	req, _ := http.NewRequest(http.MethodGet, "http://localhost:3001/user/validate/"+username+"/"+password, nil)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("FATAL err")
		log.Fatalln(err)
	}

	data, _ := ioutil.ReadAll(resp.Body)

	return string(data)
}

func validateJWT(tokenStr string) (*jwt.Token, error) {
	//TODO: PUT IN ENV
	secret := "cepic"
	return jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.WriteHeader(status)
	w.Header().Add("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}

func redirect(redirectAddr string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")

		client := &http.Client{
			Timeout: time.Second * 10,
		}

		var req *http.Request
		var data []byte
		if r.Method == http.MethodPost {
			data, _ := ioutil.ReadAll(r.Body)
			req, _ = http.NewRequest(http.MethodPost, redirectAddr+r.URL.String(), bytes.NewBuffer(data))
			req.Header.Set("Content-Type", "application/json; charset=utf-8")
		} else if r.Method == http.MethodPut {
			data, _ := ioutil.ReadAll(r.Body)
			req, _ = http.NewRequest(http.MethodPut, redirectAddr+r.URL.String(), bytes.NewBuffer(data))
			req.Header.Set("Content-Type", "application/json; charset=utf-8")
		} else if r.Method == http.MethodGet {
			req, _ = http.NewRequest(http.MethodGet, redirectAddr+r.URL.String(), nil)
		} else if r.Method == http.MethodDelete {
			req, _ = http.NewRequest(http.MethodDelete, redirectAddr+r.URL.String(), nil)
		}

		res, err := client.Do(req)
		if err != nil {
			return
		}
		defer res.Body.Close()

		data, _ = ioutil.ReadAll(res.Body)
		w.WriteHeader(res.StatusCode)
		w.Write([]byte(string(data)))
	}
}
