package pkg

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CommandType represents the type of CMS command
type CommandType string

const (
	CommandLogin    CommandType = "login"
	CommandLogout   CommandType = "logout"
	CommandHAStatus CommandType = "ha_status"
)

// CMSCommand represents a CMS command with dynamic fields
type CMSCommand struct {
	Task   string                 `json:"task"`
	Token  string                 `json:"token,omitempty"`
	Fields map[string]interface{} `json:"-"`
}

// NewCMSCommand creates a new CMS command with the specified type
func NewCMSCommand(cmdType CommandType) *CMSCommand {
	return &CMSCommand{
		Task:   string(cmdType),
		Fields: make(map[string]interface{}),
	}
}

// WithToken sets the token for the command
func (c *CMSCommand) WithToken(token string) *CMSCommand {
	c.Token = token
	return c
}

// WithField sets a field value
func (c *CMSCommand) WithField(key string, value interface{}) *CMSCommand {
	c.Fields[key] = value
	return c
}

// WithFields sets multiple field values
func (c *CMSCommand) WithFields(fields map[string]interface{}) *CMSCommand {
	for k, v := range fields {
		c.Fields[k] = v
	}
	return c
}

// ToJSON converts the command to JSON
func (c *CMSCommand) ToJSON() ([]byte, error) {
	// Create a map to hold all fields
	data := make(map[string]interface{})

	// Add task and token
	data["task"] = c.Task
	if c.Token != "" {
		data["token"] = c.Token
	}

	// Add all dynamic fields
	for k, v := range c.Fields {
		data[k] = v
	}

	return json.Marshal(data)
}

// CreateLoginCommand creates a login command
func CreateLoginCommand(id, password, clientVer string) *CMSCommand {
	return NewCMSCommand(CommandLogin).
		WithField("id", id).
		WithField("password", password).
		WithField("clientver", clientVer)
}

// CreateHAStatusCommand creates a HA status command
func CreateHAStatusCommand(token string) *CMSCommand {
	return NewCMSCommand(CommandHAStatus).
		WithToken(token)
}

// CreateLogoutCommand creates a logout command
func CreateLogoutCommand(token string) *CMSCommand {
	return NewCMSCommand(CommandLogout).
		WithToken(token)
}

// SendCommand sends a command to the CMS server
func SendCommand(command *CMSCommand, serverURL string) (*http.Response, error) {
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
