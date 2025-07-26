package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type WebClient struct {
	httpClient *http.Client
}

func NewWebClient() WebClient {
	return WebClient{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}
}

type RequestOption func(r *http.Request)

func (w WebClient) Download(url, filename string, options ...RequestOption) error {
	responseBody, err := w.makeRequest("GET", url, nil, options...)
	if err != nil {
		return err
	}
	defer func(responseBody io.ReadCloser) {
		err := responseBody.Close()
		if err != nil {

		}
	}(responseBody)

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	_, err = io.Copy(file, responseBody)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

func (w WebClient) Get(url string, options ...RequestOption) ([]byte, error) {
	responseBody, err := w.makeRequest("GET", url, nil, options...)
	if err != nil {
		return nil, err
	}
	defer func(responseBody io.ReadCloser) {
		err := responseBody.Close()
		if err != nil {

		}
	}(responseBody)

	body, err := io.ReadAll(responseBody)
	if err != nil {
		return nil, fmt.Errorf("could not read response: %w", err)
	}
	return body, nil
}

func (w WebClient) Post(url string, requestBody []byte, options ...RequestOption) ([]byte, error) {
	responseBody, err := w.makeRequest("POST", url, bytes.NewReader(requestBody), options...)
	if err != nil {
		return nil, err
	}
	defer func(responseBody io.ReadCloser) {
		err := responseBody.Close()
		if err != nil {

		}
	}(responseBody)

	body, err := io.ReadAll(responseBody)
	if err != nil {
		return nil, fmt.Errorf("could not read response: %w", err)
	}
	return body, nil
}

func WithHeader(name, value string) RequestOption {
	return func(r *http.Request) { r.Header.Add(name, value) }
}

func (w WebClient) makeRequest(method string, url string, body io.Reader, options ...RequestOption) (io.ReadCloser, error) {
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}

	for _, option := range options {
		option(request)
	}

	if os.Getenv("GITHUB_TOKEN") != "" {
		request.Header.Set("Authorization", "Bearer "+os.Getenv("GITHUB_TOKEN"))
	}

	response, err := w.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("got unsuccessful response: status code: %d, body: %s", response.StatusCode, body)
	}

	return response.Body, nil
}
