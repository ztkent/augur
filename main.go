package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ztkent/ai-util/pkg/aiutil"
	"github.com/Ztkent/augur/internal/routes"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	zlog "github.com/rs/zerolog/log"
)

/*
  Model Options:
    -openai:
	  - gpt-4-turbo-preview, aka: turbo
	  - gpt-3.5-turbo, aka: turbo35
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
	client, err := ConnectDefaultClient()
	if err != nil {
		panic(err.Error())
	}

	// Initialize router and middleware
	r := chi.NewRouter()
	// Log request and recover from panics
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Define routes
	DefineRoutes(r, &routes.Augur{
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

func DefineRoutes(r *chi.Mux, a *routes.Augur) {
	// Apply a rate limiter to all routes
	r.Use(httprate.Limit(
		50,             // requests
		60*time.Second, // per duration
		httprate.WithKeyFuncs(httprate.KeyByIP, httprate.KeyByEndpoint),
	))

	// App page
	r.Get("/", a.ServeHome())                     // Serve the landing page
	r.Post("/work", a.DoWork())                   // Generate a new prompt
	r.Post("/close", a.EmptyResponse())           // Clear an HTML div w/ HTMX
	r.Get("/download", a.Download())              // Download the prompt response
	r.Post("/switch-model", a.SwitchModel())      // Swap to another model option
	r.Post("/regenerate", a.Regenerate())         // Regenerate a given section of the prompt
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

func ConnectDefaultClient() (*aiutil.Client, error) {
	var client *aiutil.Client
	if AI_PROVIDER == "openai" {
		err := aiutil.MustLoadAPIKey(true, false)
		if err != nil {
			return nil, err
		}
		if model, ok := aiutil.IsOpenAIModel(MODEL); ok {
			zlog.Debug().Msg(fmt.Sprintf("Starting client with OpenAI-%s\n", model))
			client = aiutil.MustConnectOpenAI(model, float32(TEMPERATURE))
		} else {
			zlog.Debug().Msg(fmt.Sprintf("Starting client with OpenAI-%s\n", aiutil.GPT35Turbo))
			client = aiutil.MustConnectOpenAI(aiutil.GPT35Turbo, float32(TEMPERATURE))
		}
	} else if AI_PROVIDER == "anyscale" {
		err := aiutil.MustLoadAPIKey(false, true)
		if err != nil {
			return nil, err
		}

		if model, ok := aiutil.IsAnyscaleModel(MODEL); ok {
			zlog.Debug().Msg(fmt.Sprintf("Starting client with Anyscale-%s\n", model))
			client = aiutil.MustConnectAnyscale(model, float32(TEMPERATURE))
		} else {

			zlog.Debug().Msg(fmt.Sprintf("Starting client with Anyscale-%s\n", aiutil.CodeLlama34b))
			client = aiutil.MustConnectAnyscale(aiutil.CodeLlama34b, float32(TEMPERATURE))
		}
	} else {
		return nil, fmt.Errorf("Invalid AI provider: %s provided, select either anyscale or openai", AI_PROVIDER)
	}
	return client, nil
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
