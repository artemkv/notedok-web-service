package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
)

var baseUrl string
var port string

func init() {
	port = GetOptionalString("NOTEDOK_PORT", ":8700")
	baseUrl = fmt.Sprintf("http://127.0.0.1%s", port)
}

func TestHealthCheckIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	statusCode, _ := request(t, baseUrl+"/health")

	if statusCode != 200 {
		t.Errorf("Expected 200, actual: %d", statusCode)
	}
}

func TestLivenessCheckIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	statusCode, _ := request(t, baseUrl+"/liveness")

	if statusCode != 200 {
		t.Errorf("Expected 200, actual: %d", statusCode)
	}
}

func TestReadinessCheckIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	statusCode, _ := request(t, baseUrl+"/readiness")

	if statusCode != 200 {
		t.Errorf("Expected 200, actual: %d", statusCode)
	}
}

func TestTryAccessingRootIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	statusCode, body := request(t, baseUrl+"/")

	if statusCode != 404 {
		t.Errorf("Expected 404, actual: %d", statusCode)
	}
	if errorMessage := getErrorMessage(t, body); errorMessage != "Not found" {
		t.Errorf("Expected 'Not found', actual: %s", errorMessage)
	}
}

func TestTryAccessingNonExistingPageIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	statusCode, body := request(t, baseUrl+"/xxx")

	if statusCode != 404 {
		t.Errorf("Expected 404, actual: %d", statusCode)
	}
	if errorMessage := getErrorMessage(t, body); errorMessage != "Not found" {
		t.Errorf("Expected 'Not found', actual: %s", errorMessage)
	}
}

func TestHandleErrorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	statusCode, body := request(t, baseUrl+"/error")

	if statusCode != 500 {
		t.Errorf("Expected 404, actual: %d", statusCode)
	}
	if errorMessage := getErrorMessage(t, body); errorMessage != "Test error" {
		t.Errorf("Expected 'Test error', actual: %s", errorMessage)
	}
}

func request(t *testing.T, url string) (int, []byte) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Error creating request: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Error making request: %s", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading body: %s", err)
	}

	return resp.StatusCode, body
}

type errorResponse struct {
	ErrorText string `json:"err"`
}

func getErrorMessage(t *testing.T, body []byte) string {
	var error errorResponse
	err := json.Unmarshal(body, &error)
	if err != nil {
		t.Fatalf("Error parsing body: %s", err)
	}
	return error.ErrorText
}
