package function

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/gorilla/mux"
)

var firestoreClient *firestore.Client

func init() {
	ctx := context.Background()
	projectId := os.Getenv("GOOGLE_PROJECT_ID")
	// Remember to change the constructor in the future to use a named database
	client, err := firestore.NewClient(ctx, projectId)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	firestoreClient = client
	functions.HTTP("RESTHandler", RESTHandler)
}

// RESTHandler - Main function handling RESTful routing
func RESTHandler(w http.ResponseWriter, r *http.Request) {
	router := mux.NewRouter()

	// Define routes
	router.HandleFunc("/evaluations", storeEvaluations).Methods("POST")
	router.HandleFunc("/surveys", createSurvey).Methods("POST")
	router.HandleFunc("/surveys/{surveyId}", getSurvey).Methods("GET")
	// Serve the request
	// Wrap the router with CORS middleware
	corsHandler := enableCORS(router)
	corsHandler.ServeHTTP(w, r)
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Allow all origins
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// 📝 **Handler to create a new survey**
func createSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Parse JSON request body
	var survey map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&survey); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Ensure required fields are present
	requiredFields := []string{"quarter", "year", "evaluator", "questions", "teamMembers"}
	for _, field := range requiredFields {
		if _, exists := survey[field]; !exists {
			http.Error(w, fmt.Sprintf("Missing field: %s", field), http.StatusBadRequest)
			return
		}
	}

	// Add timestamp
	survey["createdAt"] = time.Now()

	// Store survey in Firestore with auto-generated ID
	docRef, _, err := firestoreClient.Collection("surveys").Add(ctx, survey)
	if err != nil {
		log.Printf("Error storing survey: %v", err)
		http.Error(w, "Error storing survey", http.StatusInternalServerError)
		return
	}

	// Respond with survey ID
	response := map[string]string{"surveyId": docRef.ID}
	json.NewEncoder(w).Encode(response)
}

// 🔍 **Handler to retrieve a survey by ID**
func getSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	vars := mux.Vars(r)
	surveyId := vars["surveyId"]

	// Fetch survey document
	docRef := firestoreClient.Collection("surveys").Doc(surveyId)
	docSnap, err := docRef.Get(ctx)
	if err != nil {
		http.Error(w, "Survey not found", http.StatusNotFound)
		return
	}

	// Convert Firestore document to JSON
	surveyData := docSnap.Data()
	surveyData["surveyId"] = surveyId // Include surveyId in response

	// Respond with JSON
	json.NewEncoder(w).Encode(surveyData)
}

// Handler to store evaluations in Firestore
func storeEvaluations(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Decode request body
	var evaluations []map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&evaluations); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Store each evaluation entry
	// force deploy
	for _, evaluation := range evaluations {
		teamMember, ok := evaluation["teamMember"].(string)
		if !ok || teamMember == "" {
			http.Error(w, "Missing teamMember field", http.StatusBadRequest)
			return
		}
		surveyId, ok := evaluation["surveyId"].(string)
		if !ok || surveyId == "" {
			http.Error(w, "Missing surveyId field", http.StatusBadRequest)
			return
		}
		quarter, ok := evaluation["quarter"].(string)
		if !ok || quarter == "" {
			http.Error(w, "Missing quarter field", http.StatusBadRequest)
			return
		}
		year, ok := evaluation["year"].(float64)
		if !ok || year < float64(time.Now().Year()) {
			log.Printf("year %v ", year)
			log.Printf("ok %v", ok)
			http.Error(w, "Missing year field", http.StatusBadRequest)
			return
		}

		evals, ok := evaluation["evaluations"].([]interface{})
		if !ok {
			http.Error(w, "Invalid evaluations field", http.StatusBadRequest)
			return
		}

		// Calculate average grade
		var totalGrade float64
		var count int
		for _, e := range evals {
			evalMap, ok := e.(map[string]interface{})
			if !ok {
				continue
			}
			if grade, exists := evalMap["grade"].(float64); exists {
				totalGrade += grade
				count++
			}
		}
		averageGrade := 0.0
		if count > 0 {
			averageGrade = totalGrade / float64(count)
		}

		// Prepare Firestore document
		doc := map[string]interface{}{
			"surveyId":     surveyId,
			"year":         year,
			"queater":      quarter,
			"teamMember":   teamMember,
			"evaluations":  evals, // Store full evaluations exactly as received
			"averageGrade": averageGrade,
			"submittedAt":  time.Now(), // Timestamp for easier querying
		}

		// Store under `evaluations` collection with an auto-generated ID
		_, _, err := firestoreClient.Collection("evaluations").Add(ctx, doc)
		if err != nil {
			log.Printf("Error storing evaluation for %s: %v", teamMember, err)
			http.Error(w, "Error storing evaluation", http.StatusInternalServerError)
			return
		}
	}

	// Send success response
	response := map[string]string{"message": "Evaluations stored successfully"}
	json.NewEncoder(w).Encode(response)
}
