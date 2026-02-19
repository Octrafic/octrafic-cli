package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type PostmanExporter struct{}

func (e *PostmanExporter) FileExtension() string {
	return ".json"
}

func (e *PostmanExporter) Export(req ExportRequest) error {
	collection := e.buildCollection(req)

	data, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal collection: %w", err)
	}

	if err := os.WriteFile(req.FilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (e *PostmanExporter) buildCollection(req ExportRequest) map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(req.Tests))

	for _, test := range req.Tests {
		item := map[string]interface{}{
			"name":    fmt.Sprintf("%s %s", test.Method, test.Endpoint),
			"request": e.buildRequest(test, req),
		}

		if test.Error == "" && test.StatusCode > 0 {
			item["response"] = []map[string]interface{}{
				e.buildResponse(test),
			}
		}

		items = append(items, item)
	}

	return map[string]interface{}{
		"info": map[string]interface{}{
			"name":        "Octrafic Generated Tests",
			"description": fmt.Sprintf("Generated from %s", req.BaseURL),
			"schema":      "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		"item": items,
		"variable": []map[string]interface{}{
			{
				"key":   "baseUrl",
				"value": req.BaseURL,
				"type":  "string",
			},
		},
	}
}

func (e *PostmanExporter) buildRequest(test TestData, req ExportRequest) map[string]interface{} {
	request := map[string]interface{}{
		"method": test.Method,
		"header": e.buildHeaders(test, req),
		"url": map[string]interface{}{
			"raw":  "{{baseUrl}}" + test.Endpoint,
			"host": []string{"{{baseUrl}}"},
			"path": strings.Split(strings.TrimPrefix(test.Endpoint, "/"), "/"),
		},
	}

	if test.Body != nil {
		if bodyStr, ok := test.Body.(string); ok {
			request["body"] = map[string]interface{}{
				"mode": "raw",
				"raw":  bodyStr,
				"options": map[string]interface{}{
					"raw": map[string]interface{}{
						"language": "json",
					},
				},
			}
		}
	}

	return request
}

func (e *PostmanExporter) buildHeaders(test TestData, req ExportRequest) []map[string]interface{} {
	headers := []map[string]interface{}{
		{
			"key":   "Content-Type",
			"value": "application/json",
		},
	}

	for key, value := range test.Headers {
		headers = append(headers, map[string]interface{}{
			"key":   key,
			"value": value,
		})
	}

	if test.RequiresAuth && req.AuthType != "" {
		switch req.AuthType {
		case "bearer":
			if token, ok := req.AuthData["token"]; ok {
				headers = append(headers, map[string]interface{}{
					"key":   "Authorization",
					"value": "Bearer " + token,
				})
			}
		case "apikey":
			if keyName, ok := req.AuthData["key_name"]; ok {
				if keyValue, ok := req.AuthData["key_value"]; ok {
					headers = append(headers, map[string]interface{}{
						"key":   keyName,
						"value": keyValue,
					})
				}
			}
		}
	}

	return headers
}

func (e *PostmanExporter) buildResponse(test TestData) map[string]interface{} {
	return map[string]interface{}{
		"name":   fmt.Sprintf("%d Response", test.StatusCode),
		"status": fmt.Sprintf("%d", test.StatusCode),
		"code":   test.StatusCode,
		"header": []map[string]interface{}{
			{
				"key":   "Content-Type",
				"value": "application/json",
			},
		},
		"body": test.ResponseBody,
	}
}
