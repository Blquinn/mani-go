package main

import (
	"io"
	"net/http"
)

func main() {
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		for k, v := range r.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}

		w.WriteHeader(http.StatusOK)

		io.Copy(w, r.Body)
	})

	http.ListenAndServe(":5555", nil)
}
