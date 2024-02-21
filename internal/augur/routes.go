package augur

import (
	"fmt"
	"log"
	"net/http"
	"text/template"

	"github.com/Ztkent/augur/pkg/aiclient"
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
		fmt.Println(r.Form.Get("userInput"))
		// Render the template
		tmpl, err := template.ParseFiles("internal/html/templates/augur_response.gohtml")
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, nil)
		if err != nil {
			log.Default().Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
}
