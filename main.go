package main

import (
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"strconv"
)

var version = "dev"

const page = `<!doctype html>
<html>
<head><title>mygpt-test calculator</title></head>
<body>
<h1>mygpt-test Calculator</h1>
<form action="/calc" method="get">
<input name="a" placeholder="number a">
<select name="op">
<option value="+">+</option>
<option value="-">-</option>
<option value="*">*</option>
<option value="/">/</option>
</select>
<input name="b" placeholder="number b">
<button>Calculate</button>
</form>
<p>{{.}}</p>
</body>
</html>`

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t := template.Must(template.New("page").Parse(page))
		t.Execute(w, "")
	})

	http.HandleFunc("/calc", func(w http.ResponseWriter, r *http.Request) {
		a, _ := strconv.ParseFloat(r.URL.Query().Get("a"), 64)
		b, _ := strconv.ParseFloat(r.URL.Query().Get("b"), 64)
		op := r.URL.Query().Get("op")
		var result float64
		switch op {
		case "+": result = a + b
		case "-": result = a - b
		case "*": result = a * b
		case "/":
			if b != 0 { result = a / b }
		}
		http.Redirect(w, r, fmt.Sprintf("/?result=%v", result), http.StatusFound)
	})

	fmt.Println("mygpt-test web calculator")
	fmt.Println("version:", version)
	fmt.Println("go:", runtime.Version())
	fmt.Println("os/arch:", runtime.GOOS, runtime.GOARCH)
	fmt.Println("listen: http://0.0.0.0:8080")
	http.ListenAndServe(":8080", nil)
}
