package routes

import (
	"net/http"

	"github.com/Ztkent/augur/pkg/aiclient"
)

type Augur struct {
	Client *aiclient.Client
}

func (a *Augur) ServeHome() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/html/home.html")
	}
}
