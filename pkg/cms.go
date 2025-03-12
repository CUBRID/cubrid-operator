package pkg

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"
)

type Command struct {
	Task  string                 `json:"task"`
	Token string                 `json:"token,omitempty"`
	Extra map[string]interface{} `json:"extra,omitempty"`
}

var CommandTemplates = map[string]map[string]string{
	"login": {
		"task":      "login",
		"id":        "",     // id is dynamic
		"password":  "",     // password is dynamic
		"clientver": "11.4", // fixed client version for login
	},
	"ha_status": {
		"task":  "ha_status",
		"token": "", // Token is dynamic
	},
}

type CommandBuilder struct {
	Token string
}

func NewCommandBuilder(token string) *CommandBuilder {
	return &CommandBuilder{Token: token}
}

func (cb *CommandBuilder) CreateCommand(taskName string, extra map[string]interface{}) (*Command, error) {
	template, exists := CommandTemplates[taskName]
	if !exists {
		return nil, fmt.Errorf("unknown command: %s", taskName)
	}

	cmd := &Command{
		Task:  template["task"],
		Token: cb.Token,
		Extra: make(map[string]interface{}),
	}

	for key, value := range extra {
		cmd.Extra[key] = value
	}

	return cmd, nil
}

func (c *Command) ToJSON() ([]byte, error) {
	payload := map[string]interface{}{
		"task": c.Task,
	}

	if c.Token != "" {
		payload["token"] = c.Token
	}

	for key, value := range c.Extra {
		payload[key] = value
	}
	return json.Marshal(payload)
}

func SendCommand(command *Command, serverURL string) (*http.Response, error) {
	cmdJSON, err := command.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("error marshaling command: %v", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			ForceAttemptHTTP2: false,
		},
	}

	req, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(cmdJSON))
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %v", err)
	}
	req.Header.Set("Accept", "Text/plain;charset=utf-8")

	// dump, err := httputil.DumpRequestOut(req, true)
	// if err != nil {
	// 	return nil, fmt.Errorf("error dumping request: %v", err)
	// }
	// fmt.Printf("Request:\n%s\n", string(dump))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending command: %v", err)
	}

	// respDump, err := httputil.DumpResponse(resp, true)
	// if err != nil {
	// 	return nil, fmt.Errorf("error dumping response: %v", err)
	// }
	// fmt.Printf("Response:\n%s\n", string(respDump))

	return resp, nil
}
