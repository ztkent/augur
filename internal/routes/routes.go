package routes

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"unicode"

	"github.com/Ztkent/augur/internal/prompts"
	"github.com/Ztkent/augur/pkg/aiclient"
	"github.com/google/uuid"
)

const (
	INTRO_PROMPT    = "INTRO_PROMPT"
	PT_PROMPT       = "PT_PROMPT"
	RULES_PROMPT    = "RULES_PROMPT"
	REMINDER_PROMPT = "REMINDER_PROMPT"
	APPNAME_PROMPT  = "APPNAME_PROMPT"
	MAX_ATTEMPTS    = 3
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

func (a *Augur) EnsureUUIDHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie("uuid")
		if err == http.ErrNoCookie {
			// Cookie does not exist, set it
			token := uuid.New().String()
			http.SetCookie(w, &http.Cookie{
				Name:     "uuid",
				Value:    token,
				HttpOnly: true,
				Secure:   true, // Set to true if your site uses HTTPS
				SameSite: http.SameSiteStrictMode,
			})
		} else if err != nil {
			// Some other error occurred
			http.Error(w, "Failed to read cookie", http.StatusInternalServerError)
		}
	}
}

func (a *Augur) Download() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid, err := getRequestCookie(r, "uuid")
		if err != nil {
			http.Error(w, "User UUID not found", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=prompt.md"))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, fmt.Sprintf("temp/response_%s.md", uuid))
	}
}

func (a *Augur) SwitchModel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := getRequestCookie(r, "uuid")
		if err != nil {
			http.Error(w, "User UUID not found", http.StatusBadRequest)
			return
		}
		r.ParseForm()
		modelVal := r.Form.Get("modelDropdown")
		if modelVal == "" {
			http.Error(w, "No model selected", http.StatusBadRequest)
			return
		}
		provider := strings.Split(modelVal, ",")[0]
		model := strings.Split(modelVal, ",")[1]

		if provider == "openai" {
			if model, ok := aiclient.IsOpenAIModel(model); ok {
				fmt.Println(fmt.Sprintf("Swapping client to OpenAI-%s\n", model))
				a.Client = aiclient.MustConnectOpenAI(model, float32(a.Client.Temperature))
			} else {
				http.Error(w, "Invalid OpenAI model", http.StatusBadRequest)
				return
			}
		} else if provider == "anyscale" {
			if model, ok := aiclient.IsAnyscaleModel(model); ok {
				fmt.Println(fmt.Sprintf("Swapping client to Anyscale-%s\n", model))
				a.Client = aiclient.MustConnectAnyscale(model, float32(a.Client.Temperature))
			} else {
				http.Error(w, "Invalid Anyscale model", http.StatusBadRequest)
				return
			}
		} else {
			http.Error(w, "Invalid AI provider", http.StatusBadRequest)
			return
		}
	}
}

type Prompt struct {
	Introduction string
	Pretraining  string
	Rules        string
	Important    string
	AppName      string
	RequestLog   string
}

func (a *Augur) DoWork() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		// Validate the UUID
		uuid, err := getRequestCookie(r, "uuid")
		if err != nil {
			log.Default().Println(err)
			serveToast(w, "Failed to read UUID")
			return
		}

		// Grab the user input
		userInput := r.Form.Get("userInput")
		if userInput == "" {
			log.Default().Println("No App Idea provided")
			serveToast(w, "No App Idea provided")
			return
		} else if len(userInput) > 75 {
			log.Default().Println("App Idea too long")
			serveToast(w, "App Idea too long")
			return
		}
		userInput = "App Idea: " + userInput

		// Set the custom temperature
		tempInput, err := strconv.ParseFloat(r.Form.Get("tempInput"), 32)
		if err != nil {
			log.Default().Println(err)
			serveToast(w, "Invalid temperature setting")
			return
		}
		requestLog := fmt.Sprint(userInput + " - Model: " + a.Client.Model + " - " + fmt.Sprintf("Temp: %f", tempInput))
		a.Client.SetTemperature(float32(tempInput))

		// Generate the each piece of the response concurrently
		attempts := 0
		complete := false
		errChan := make(chan error, 5)
		responsePrompt := Prompt{}
		for !complete {
			if attempts > MAX_ATTEMPTS {
				serveToast(w, "Failed to generate a valid response")
				return
			}

			// Build the 'Introduction' piece of the response
			wg := sync.WaitGroup{}
			wg.Add(5)
			go func() {
				defer wg.Done()
				var err error
				responsePrompt.Introduction, err = a.completeIntroSection(r.Context(), userInput)
				if err != nil {
					errChan <- err
					return
				}
			}()
			// Build the 'Pretraining' piece of the response
			go func() {
				defer wg.Done()
				var err error
				responsePrompt.Pretraining, err = a.completeListSection(r.Context(), userInput, PT_PROMPT, 4, 6)
				if err != nil {
					errChan <- err
					return
				}
			}()
			// Build the 'Rules' piece of the response
			go func() {
				defer wg.Done()
				var err error
				responsePrompt.Rules, err = a.completeListSection(r.Context(), "", RULES_PROMPT, 4, 6)
				if err != nil {
					errChan <- err
					return
				}
			}()
			// Build the 'Important' piece of the response
			go func() {
				defer wg.Done()
				var err error
				responsePrompt.Important, err = a.completeListSection(r.Context(), "", REMINDER_PROMPT, 2, 4)
				if err != nil {
					errChan <- err
					return
				}
			}()
			// Generate an app name
			go func() {
				defer wg.Done()
				var err error
				responsePrompt.AppName, err = a.generateAppName(r.Context(), userInput)
				if err != nil {
					errChan <- err
					return
				}
			}()

			// Wait for all the pieces to be built
			wg.Wait()
			// Check for any errors
			select {
			case err := <-errChan:
				log.Default().Println(err)
				serveToast(w, err.Error())
				attempts++
				continue
			default:
			}

			resultPrompt := responsePrompt.Introduction + "\n\n"
			resultPrompt += "## Pretraining\n"
			resultPrompt += responsePrompt.Pretraining + "\n\n"
			resultPrompt += "## Rules\n"
			resultPrompt += responsePrompt.Rules + "\n\n"
			resultPrompt += "## Important\n"
			resultPrompt += responsePrompt.Important + "\n\n"
			resultPrompt += requestLog + "\n"
			fmt.Println(resultPrompt)

			// Review this prompt for language and completeness.
			words := strings.Fields(resultPrompt)
			if len(words) < 100 {
				fmt.Println("Prompt is too short, trying again")
				attempts++
				continue
			}
			complete = true
		}

		// Write the response to the temp folder
		responsePrompt.RequestLog = requestLog
		err = writeResults(uuid, responsePrompt)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Render the template
		tmpl, err := template.ParseFiles("internal/html/templates/augur_response.gohtml")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, responsePrompt)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
}

var blockedWords = map[string]bool{
	"You:":  true,
	"AI:":   true,
	"User:": true,
	"LLM":   true,
	"```":   true,
}

func (a *Augur) completeIntroSection(ctx context.Context, userInput string) (string, error) {
	attempts := 0
	for {
		if attempts > MAX_ATTEMPTS {
			return "", fmt.Errorf("Failed to generate a valid intro")
		}

		convo := aiclient.NewConversation(prompts.GetPrompt(INTRO_PROMPT), 0, 0)
		// convo.SeedConversation()
		res, err := a.Client.SendCompletionRequest(ctx, convo, userInput)
		if err != nil {
			return "", err
		}
		res = strings.TrimFunc(res, func(r rune) bool {
			return r == '-' || r == '*' || unicode.IsDigit(r) || r == '[' || r == ']' || r == '.' || r == '`' || r == ' ' || r == '\n'
		})
		if res == "" {
			attempts++
			continue
		}

		// Check if the line contains a blocked word
		reset := false
		for blockedWord := range blockedWords {
			if strings.Contains(res, blockedWord) {
				reset = true
				break
			}
		}
		if reset {
			attempts++
			continue
		}
		return res, nil
	}
}

func (a *Augur) completeListSection(ctx context.Context, userInput string, prompt string, minResponseLength int, maxResponseLength int) (string, error) {
	attempts := 0
	for {
		if attempts > MAX_ATTEMPTS {
			return "", fmt.Errorf("Failed to generate a valid " + prompt + " list")
		}

		convo := aiclient.NewConversation(prompts.GetPrompt(prompt), 0, 0)
		// convo.SeedConversation()
		var err error
		res, err := a.Client.SendCompletionRequest(ctx, convo, userInput)
		if err != nil {
			return "", err
		}

		// Split the response by newline
		lines := strings.Split(res, "\n")
		// Iterate over the lines and remove leading characters
		outputLines := make([]string, 0)
		for i, line := range lines {
			line = strings.TrimFunc(line, func(r rune) bool {
				return r == '-' || r == '*' || unicode.IsDigit(r) || r == '[' || r == ']' || r == '.' || r == '`' || r == ' ' || r == '\n'
			})
			if line == "" {
				continue
			}
			// Check if the line contains a blocked word
			for blockedWord := range blockedWords {
				if strings.Contains(lines[i], blockedWord) {
					continue
				}
			}

			line = "- " + line
			outputLines = append(outputLines, line)
		}

		// Ensure a valid response, block any words we know are bad
		if len(outputLines) < minResponseLength || len(outputLines) > maxResponseLength {
			attempts++
			continue
		}

		return strings.Join(outputLines, "<br>\n"), nil
	}
}

func (a *Augur) generateAppName(ctx context.Context, resultPrompt string) (string, error) {
	attempts := 0
	for {
		if attempts > MAX_ATTEMPTS {
			return "", fmt.Errorf("Failed to generate a valid app name")
		}

		convo := aiclient.NewConversation(prompts.GetPrompt(APPNAME_PROMPT), 0, 0)
		res, err := a.Client.SendCompletionRequest(ctx, convo, resultPrompt)
		if err != nil {
			return "", err
		}
		res = strings.TrimSpace(res)
		res = strings.Split(res, "\n")[0]

		// Ensure the response is more than 1 word, and less than 5 words
		words := strings.Fields(res)
		if len(words) < 1 || len(words) > 5 {
			attempts++
			continue
		}

		// Trim any " OR / OR ' from the start/end of the response
		res = strings.Trim(res, "/")
		res = strings.Trim(res, "'")
		res = strings.Trim(res, "\"")
		res = strings.Trim(res, "-")
		res = strings.TrimSpace(res)

		return res, nil
	}
}

func writeResults(uuid string, responsePrompt Prompt) error {
	f, err := os.OpenFile(fmt.Sprintf("temp/response_%s.md", uuid), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Default().Println(err)
		return err
	}
	defer f.Close()
	prompt := responsePrompt.Introduction + "\n\n## Pretraining\n" + responsePrompt.Pretraining + "\n\n## Rules\n" + responsePrompt.Rules + "\n\n## Important\n" + responsePrompt.Important
	prompt = strings.ReplaceAll(prompt, "<br>", "")
	if _, err := f.WriteString(prompt); err != nil {
		log.Default().Println(err)
		return err
	}
	return nil
}

func getRequestCookie(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err == http.ErrNoCookie {
		return "", fmt.Errorf("Cookie not found")
	}
	return cookie.Value, nil
}

type Toast struct {
	ToastContent string
	Border       string
}

func serveToast(w http.ResponseWriter, message string) {
	// Render the crawl_status template, which displays the toast
	tmpl, err := template.ParseFiles("internal/html/templates/toast.gohtml")
	if err != nil {
		log.Default().Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	toast := &Toast{ToastContent: message, Border: "border-red-200"}
	err = tmpl.Execute(w, toast)
	if err != nil {
		log.Default().Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return
}

func logForm(r *http.Request) {
	r.ParseForm()
	for key, values := range r.Form {
		for _, value := range values {
			log.Printf("Form key: %s, value: %s\n", key, value)
		}
	}
}
