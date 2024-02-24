package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ztkent/augur/internal/routes"
	"github.com/Ztkent/augur/pkg/aiclient"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	zlog "github.com/rs/zerolog/log"
)

/*
  Model Options:
    -openai:
	  - gpt-3.5-turbo, aka: turbo
	-anyscale:
	  - mistralai/Mistral-7B-Instruct-v0.1, aka: m7b
	  - mistralai/Mixtral-8x7B-Instruct-v0.1, aka: m8x7b
	  - meta-llama/Llama-2-7b-chat-hf, aka: l7b
	  - meta-llama/Llama-2-13b-chat-hf, aka: l13b
	  - meta-llama/Llama-2-70b-chat-hf, aka: l70b
	  - codellama/CodeLlama-34b-Instruct-hf, aka: cl34b
	  - codellama/CodeLlama-70b-Instruct-hf, aka: cl70b
*/

const ( // Default values
	// AI_PROVIDER = "openai"
	// MODEL       = "turbo"
	AI_PROVIDER = "anyscale"
	MODEL       = "m8x7b"
	TEMPERATURE = 0.7
)

func main() {
	// Load the API key and connect to the AI provider
	checkRequiredEnvs()
	var client *aiclient.Client
	if AI_PROVIDER == "openai" {
		err := aiclient.MustLoadAPIKey(true, false)
		if err != nil {
			fmt.Printf("Failed to load OpenAI API key: %s\n", err)
			return
		}
		// Connect to the OpenAI Client with the given model
		if model, ok := aiclient.IsOpenAIModel(MODEL); ok {
			zlog.Debug().Msg(fmt.Sprintf("Starting client with OpenAI-%s\n", model))
			client = aiclient.MustConnectOpenAI(model, float32(TEMPERATURE))
		} else {
			// Default to GPT-3.5 Turbo
			zlog.Debug().Msg(fmt.Sprintf("Starting client with OpenAI-%s\n", aiclient.GPT35Turbo))
			client = aiclient.MustConnectOpenAI(aiclient.GPT35Turbo, float32(TEMPERATURE))
		}
	} else if AI_PROVIDER == "anyscale" {
		err := aiclient.MustLoadAPIKey(false, true)
		if err != nil {
			zlog.Error().AnErr("Failed to load Anyscale API key", err)
			return
		}
		// Connect to the Anyscale Client with the given model
		if model, ok := aiclient.IsAnyscaleModel(MODEL); ok {
			zlog.Debug().Msg(fmt.Sprintf("Starting client with Anyscale-%s\n", model))
			client = aiclient.MustConnectAnyscale(model, float32(TEMPERATURE))
		} else {
			// Default to CodeLlama
			zlog.Debug().Msg(fmt.Sprintf("Starting client with Anyscale-%s\n", aiclient.CodeLlama34b))
			client = aiclient.MustConnectAnyscale(aiclient.CodeLlama34b, float32(TEMPERATURE))
		}
	} else {
		fmt.Println(fmt.Sprintf("Invalid AI provider: %s provided, select either anyscale or openai", AI_PROVIDER))
		return
	}

	// Initialize router and middleware
	r := chi.NewRouter()
	// Log request and recover from panics
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Define routes
	defineRoutes(r, &routes.Augur{
		Client: client,
	})

	// Start server
	fmt.Println("Server is running on port " + os.Getenv("APP_PORT"))
	if os.Getenv("ENV") == "dev" {
		log.Fatal(http.ListenAndServe(":"+os.Getenv("APP_PORT"), r))
	}
	log.Fatal(http.ListenAndServeTLS(":"+os.Getenv("APP_PORT"), os.Getenv("CERT_PATH"), os.Getenv("CERT_KEY_PATH"), r))
	return
}

func defineRoutes(r *chi.Mux, a *routes.Augur) {
	// Apply a rate limiter to all routes
	r.Use(httprate.Limit(
		50,             // requests
		60*time.Second, // per duration
		httprate.WithKeyFuncs(httprate.KeyByIP, httprate.KeyByEndpoint),
	))

	// App page
	r.Get("/", a.ServeHome())
	r.Post("/work", a.DoWork())
	r.Post("/close", a.EmptyResponse())
	r.Get("/download", a.Download())
	r.Post("/switch-model", a.SwitchModel())
	r.Post("/regenerate", a.Regenerate())
	r.Post("/ensure-uuid", a.EnsureUUIDHandler()) // Make sure every active user is assigned a UUID

	// Serve static files
	workDir, _ := os.Getwd()
	filesDir := filepath.Join(workDir, "internal", "html", "img")
	FileServer(r, "/img", http.Dir(filesDir))
	FileServer(r, "/favicon.ico", http.Dir(filesDir))
}

func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	r.Get(path+"*", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix(path, http.FileServer(root)).ServeHTTP(w, r)
	})
}

func checkRequiredEnvs() {
	envs := []string{
		"APP_PORT",
		"INTRO_PROMPT",
		"PT_PROMPT",
		"RULES_PROMPT",
		"REMINDER_PROMPT",
		"APPNAME_PROMPT",
	}
	for _, env := range envs {
		if value := os.Getenv(env); value == "" {
			log.Fatalf("%s environment variable is not set", env)
		}
	}
}
