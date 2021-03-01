package main

import "encoding/json"

type ManiJSONResponseWrapper struct {
	Error    *ManiJSONError    `json:"error,omitempty"`
	Response *ManiJSONResponse `json:"response,omitempty"`
}

type ManiBodyEnum struct {
	Bytes []byte          `json:"bytes,omitempty"`
	JSON  json.RawMessage `json:"json,omitempty"`
}

type ManiJSONResponse struct {
	StatusCode int          `json:"statusCode"`
	Headers    [][]string   `json:"headers"`
	Body       ManiBodyEnum `json:"body"`
}

type ManiJSONError struct {
	Detail string `json:"detail"`
}
