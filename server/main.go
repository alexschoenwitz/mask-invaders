package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	// Replace "your-module/proto" with your actual module path

	"google.golang.org/protobuf/proto"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// 2. Unmarshal the Protobuf message
	req := &proto.HelloRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		http.Error(w, "Failed to parse proto", http.StatusBadRequest)
		return
	}

	// 3. Create a response message
	res := &proto.HelloResponse{
		Greeting: fmt.Sprintf("Hello, %s! Your Go server is working.", req.GetName()),
	}

	// 4. Marshal the response back to binary
	responseBytes, err := proto.Marshal(res)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	// 5. Send it back
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Write(responseBytes)
}

func main() {
	http.HandleFunc("/hello", helloHandler)

	fmt.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
