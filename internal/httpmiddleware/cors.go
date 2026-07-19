package httpmiddleware

import "net/http"

// CORS returns middleware that sets CORS headers for allowedOrigin and
// short-circuits preflight OPTIONS requests. gorilla/mux routes
// registered with .Methods(http.MethodPost) etc. don't handle OPTIONS
// automatically — without this, browser preflight requests hit no
// matching route and fall through to mux's default 405.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
