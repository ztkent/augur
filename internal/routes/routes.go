package routes

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"text/template"
	"unicode"

	"github.com/Ztkent/augur/internal/prompts"
	"github.com/Ztkent/augur/pkg/aiclient"
)

const (
	INTRO_PROMPT = "INTRO_PROMPT"
	PT_PROMPT    = "PT_PROMPT"
	RULES_PROMPT = "RULES_PROMPT"
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

		// Build the 'Introduction' piece of the response
		introResponse, err := a.completeIntroSection(r.Context(), userInput)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Build the 'Pretraining' piece of the response
		ptResponse, err := a.completeListSection(r.Context(), userInput, PT_PROMPT)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Build the 'Rules' piece of the response
		rulesResponse, err := a.completeListSection(r.Context(), "", RULES_PROMPT)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resultPrompt := ""
		resultPrompt += introResponse + "\n"
		resultPrompt += ptResponse + "\n"
		resultPrompt += rulesResponse

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
func (a *Augur) completeIntroSection(ctx context.Context, userInput string) (string, error) {
	convo := aiclient.NewConversation(prompts.GetPrompt(INTRO_PROMPT), 0, 0)
	// convo.SeedConversation()
	res, err := a.Client.SendCompletionRequest(ctx, convo, userInput)
	if err != nil {
		log.Default().Println(err)
		return "", err
	}
	return res, nil
}

func (a *Augur) completeListSection(ctx context.Context, userInput string, prompt string) (string, error) {
	var res string
	complete := false
	for !complete {
		convo := aiclient.NewConversation(prompts.GetPrompt(prompt), 0, 0)
		// convo.SeedConversation()
		var err error
		res, err = a.Client.SendCompletionRequest(ctx, convo, userInput)
		if err != nil {
			log.Default().Println(err)
			return "", err
		}

		// Split the response by newline
		lines := strings.Split(res, "\n")
		// Iterate over the lines and remove leading characters
		for i := range lines {
			lines[i] = strings.TrimSpace(lines[i])
			lines[i] = strings.TrimLeftFunc(lines[i], func(r rune) bool {
				return r == '-' || r == '*' || unicode.IsDigit(r) || r == ' '
			})
			lines[i] = strings.TrimLeftFunc(lines[i], func(r rune) bool {
				return r == '.' || r == ' '
			})
			lines[i] = "- " + lines[i]
		}
		// Ensure a valid response, block any words we know are bad
		if len(lines) < 3 {
			continue
		}
		complete = true
		res = strings.Join(lines, "\n")
	}
	return res, nil
}

func logForm(r *http.Request) {
	r.ParseForm()
	for key, values := range r.Form {
		for _, value := range values {
			log.Printf("Form key: %s, value: %s\n", key, value)
		}
	}
}
