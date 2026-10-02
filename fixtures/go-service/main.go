package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "Repository running.") })
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe("127.0.0.1:"+port, nil))
}
