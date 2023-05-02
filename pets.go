package pets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
)

var breedSource = "external"

const dataFilePath = "path/to/breed/data"

// Represents an HTTP REST server for Pet CRUD.
type PetServer struct {
	petDB   *CloudSqlDB        // stores pet details
	breedDB map[string]*Breed  // caches breed details, keyed by ID
	r       *renderer.Renderer // JSON renderer for HTTP request/response
	h       *http.Client       // HTTP client to be used for requesting data off server
}

func NewPetServer(ctx context.Context) (*PetServer, error) {
	logger := logging.FromContext(ctx)

	s := new(PetServer)

	// Make a new renderer for rendering json.
	ren, err := renderer.New(ctx, nil,
		renderer.WithOnError(func(err error) {
			logger.Errorw("failed to render", "error", err)
		}))
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	s.r = ren
	s.petDB = ConnectToCloudSqlDB(ctx /*...*/)
	s.h = &http.Client{Timeout: 30 * time.Second}

	breeds := s.getBreeds(ctx)
	s.breedDB = make(map[string]*Breed, len(breeds))
	for _, breed := range breeds {
		s.breedDB[breed.ID] = breed
	}

	return s, nil
}

func (s *PetServer) handlePetGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		s.r.RenderJSON(w, http.StatusInternalServerError, fmt.Errorf("id missing"))
		return
	}
	resp, ok := s.petDB.Select(id)
	if !ok {
		s.r.RenderJSON(w, http.StatusNotFound, fmt.Errorf("id not found: %v", id))
		return
	}
	s.r.RenderJSON(w, http.StatusOK, resp)
}

func (s *PetServer) handlePetCreate(w http.ResponseWriter, r *http.Request) {
	var cpr CreatePetRequest
	if err := json.NewDecoder(r.Body).Decode(&cpr); err != nil {
		s.r.RenderJSON(w, http.StatusBadRequest, err)
		return
	}

	if details, contains := s.breedDB[cpr.BreedID]; !contains {
		s.r.RenderJSON(w, http.StatusNotFound, fmt.Errorf("breed id not found: %v", cpr.BreedID))
		return
	}

	p := Pet{
		ID:           fmt.Sprint(cpr.BreedID, time.Now().UnixMilli()),
		Name:         cpr.Name,
		Photo:        cpr.Photo,
		BreedDetails: details,
		CreateTime:   time.Now(),
	}
	s.petDB.Insert(p.ID, &p)
	w.Header().Set("Location", "/pets/"+p.ID)
	s.r.RenderJSON(w, http.StatusCreated, p)
}

func (s *Server) getBreeds(ctx context.Context) []*Breed {
	logger := logging.FromContext(ctx)

	var breedData []byte
	if ctx.Value(breedSource) == "test" {
		jsonFilePath, ok := ctx.Value(dataFilePath).(string)
		jsonFile, err := os.Open(jsonFilePath)
		if !ok || err != nil {
			logger.Errorw("unable to open json file", "error", err)
			return nil
		}
		defer jsonFile.Close()

		logger.Debugw("loading breed data from json file", "file", jsonFilePath)
		body, err := io.ReadAll(io.LimitReader(jsonFile, 1024*1000))
		if err != nil {
			logger.Errorw("unable to parse json file", "error", err)
			return nil
		}
		breedData = body
	} else {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://api.petsite.fake/breeds", nil)
		if err != nil {
			logger.Errorw("unable to create request context to retrieve pet breeds", "error", err)
			return nil
		}

		resp, err := s.h.Do(req)
		if err != nil {
			logger.Errorw("unable to retrieve pet breeds", "error", err)
			return nil
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1000))
		if err != nil {
			logger.Errorw("unable to parse response", "error", err)
			return nil
		}
		breedData = body
	}

	var result []*Breeds
	if err := json.Unmarshal(breedData, &result); err != nil {
		logger.Errorw("cannot unmarshal JSON response", "error", err)
		return nil
	}

	return result
}
