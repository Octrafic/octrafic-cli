package exporter

import (
	"fmt"
	"os"
	"strings"
)

type PytestExporter struct{}

func (e *PytestExporter) FileExtension() string {
	return ".py"
}

func (e *PytestExporter) Export(req ExportRequest) error {
	var script strings.Builder

	script.WriteString("import requests\n")
	script.WriteString("import pytest\n\n")
	fmt.Fprintf(&script, "BASE_URL = \"%s\"\n\n", req.BaseURL)

	if req.AuthType != "" {
		script.WriteString("# Authentication configuration\n")
		switch req.AuthType {
		case "bearer":
			if token, ok := req.AuthData["token"]; ok {
				fmt.Fprintf(&script, "AUTH_TOKEN = \"%s\"\n", token)
			}
		case "apikey":
			if keyName, ok := req.AuthData["key_name"]; ok {
				if keyValue, ok := req.AuthData["key_value"]; ok {
					fmt.Fprintf(&script, "API_KEY_NAME = \"%s\"\n", keyName)
					fmt.Fprintf(&script, "API_KEY_VALUE = \"%s\"\n", keyValue)
				}
			}
		case "basic":
			if username, ok := req.AuthData["username"]; ok {
				if password, ok := req.AuthData["password"]; ok {
					fmt.Fprintf(&script, "AUTH_USER = \"%s\"\n", username)
					fmt.Fprintf(&script, "AUTH_PASS = \"%s\"\n", password)
				}
			}
		}
		script.WriteString("\n")
	}

	for i, test := range req.Tests {
		funcName := e.buildFunctionName(test, i)
		fmt.Fprintf(&script, "def %s():\n", funcName)
		fmt.Fprintf(&script, "    \"\"\"%s %s\"\"\"\n", test.Method, test.Endpoint)
		fmt.Fprintf(&script, "    url = BASE_URL + \"%s\"\n", test.Endpoint)

		script.WriteString("    headers = {\"Content-Type\": \"application/json\"")
		for key, value := range test.Headers {
			fmt.Fprintf(&script, ", \"%s\": \"%s\"", key, value)
		}
		script.WriteString("}\n")

		if test.RequiresAuth && req.AuthType != "" {
			switch req.AuthType {
			case "bearer":
				script.WriteString("    headers[\"Authorization\"] = f\"Bearer {AUTH_TOKEN}\"\n")
			case "apikey":
				script.WriteString("    headers[API_KEY_NAME] = API_KEY_VALUE\n")
			}
		}

		if req.AuthType == "basic" && test.RequiresAuth {
			script.WriteString("    auth = (AUTH_USER, AUTH_PASS)\n")
		} else {
			script.WriteString("    auth = None\n")
		}

		method := strings.ToLower(test.Method)
		if test.Body != nil {
			if bodyStr, ok := test.Body.(string); ok {
				escapedBody := strings.ReplaceAll(bodyStr, "\"", "\\\"")
				fmt.Fprintf(&script, "    data = \"\"\"%s\"\"\"\n", escapedBody)
				fmt.Fprintf(&script, "    response = requests.%s(url, headers=headers, data=data, auth=auth)\n", method)
			} else {
				fmt.Fprintf(&script, "    response = requests.%s(url, headers=headers, auth=auth)\n", method)
			}
		} else {
			fmt.Fprintf(&script, "    response = requests.%s(url, headers=headers, auth=auth)\n", method)
		}

		script.WriteString("\n")

		if test.StatusCode > 0 {
			fmt.Fprintf(&script, "    assert response.status_code == %d\n", test.StatusCode)
		} else {
			script.WriteString("    assert response.status_code < 500\n")
		}

		if test.Error != "" {
			fmt.Fprintf(&script, "    # Note: Test failed with error: %s\n", test.Error)
		}

		script.WriteString("\n\n")
	}

	if err := os.WriteFile(req.FilePath, []byte(script.String()), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (e *PytestExporter) buildFunctionName(test TestData, index int) string {
	method := strings.ToLower(test.Method)
	endpoint := strings.ReplaceAll(test.Endpoint, "/", "_")
	endpoint = strings.ReplaceAll(endpoint, "{", "")
	endpoint = strings.ReplaceAll(endpoint, "}", "")
	endpoint = strings.Trim(endpoint, "_")

	if endpoint == "" {
		return fmt.Sprintf("test_%s_%d", method, index+1)
	}

	return fmt.Sprintf("test_%s_%s", method, endpoint)
}
