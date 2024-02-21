package routes

import (
	"fmt"
	"log"
	"net/http"
	"text/template"

	"github.com/Ztkent/augur/internal/prompts"
	"github.com/Ztkent/augur/pkg/aiclient"
)

const (
	INTRO_PROMPT = "INTRO_PROMPT"
)

type Augur struct {
	Client *aiclient.Client
}

func (a *Augur) EmptyResponse() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	}
}

func (a *Augur) ServeHome() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/html/home.html")
	}
}

func (a *Augur) DoWork() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		userInput := r.Form.Get("userInput")
		fmt.Println(userInput)

		// Gather the context from the user. Did they give us a good prompt? Are we in an ok place?
		// If not, ask the user to repeat their query.
		if userInput == "" {
			http.Error(w, "No user input", http.StatusBadRequest)
			return
		}

		// We are going to build a response to the user's query in 5 steps:
		// Introduction
		// # Pretraining
		// ## Example Prompt and Response
		// ## Rules
		// ## Important

		// For each step, we will generate a piece of the response.
		// We will review this piece to ensure it matches our structure and makes sense.
		// Validate that the result is good. If not, try generating it from scratch again.

		// Once we have all 4 pieces, we will combine them into a single response
		// We will review this prompt for language and completness.
		// Return this prompt to the user.
		// Accept feedback. Probably via /feedback which can take the entire response as a parameter.

		resultPrompt := ""
		// Build the 'Introduction' piece of the response
		complete := false
		for !complete {
			introConversation := aiclient.NewConversation(prompts.GetPrompt(INTRO_PROMPT), 0, 0)
			// introConversation.SeedConversation()
			res, err := a.Client.SendCompletionRequest(r.Context(), introConversation, userInput)
			if err != nil {
				log.Default().Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Validate the response
			complete = true
			resultPrompt += res
		}

		// Render the template
		tmpl, err := template.ParseFiles("internal/html/templates/augur_response.gohtml")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, resultPrompt)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
}

func logForm(r *http.Request) {
	r.ParseForm()
	for key, values := range r.Form {
		for _, value := range values {
			log.Printf("Form key: %s, value: %s\n", key, value)
		}
	}
}
