package gateway

import (
	"net/http"
	"strings"
)

func RedirectToMovieAPI(w http.ResponseWriter, r *http.Request) {

	if strings.HasPrefix(r.URL.Path, "/movieAPI") {
		http.NotFound(w, r)
		return
	}

	path := "/movieAPI" + r.URL.Path
	http.Redirect(w, r, path, http.StatusTemporaryRedirect)
}
