package server

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/washsky/mygpt-test/internal/calculator"
)

func Start(version string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "mygpt-test web server\n")
	})

	http.HandleFunc("/calc", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, calculator.Calculate(1, 2, "+"))
	})

	fmt.Println("mygpt-test")
	fmt.Println("version:", version)
	fmt.Println("go:", runtime.Version())
	fmt.Println("listen: http://0.0.0.0:8080")
	http.ListenAndServe(":8080", nil)
}
