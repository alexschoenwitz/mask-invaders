package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"google.golang.org/protobuf/encoding/protojson"
)

// Client simplifies access to the HTTP API of the Mask Invaders server
type Client struct {
	baseURL    string
	httpClient *http.Client

	token    string
	playerID string
}

// NewClient creates a new Client instance
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Register(name string, gameID string) (string, error) {
	reqBody := &api.RegisterRequest{
		GameId: gameID,
		Name:   name,
	}
	jsonData, err := protojson.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(c.baseURL+"/v1/games/"+gameID+"/register", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	var regResp api.RegisterResponse
	if err := protojson.Unmarshal(body, &regResp); err != nil {
		return "", err
	}
	c.token = regResp.Token
	c.playerID = regResp.Id
	fmt.Printf("got token: %v\n", c.token)

	return regResp.Id, nil
}

func (c *Client) StartGame(gameID string) error {
	reqBody := &api.StartGameRequest{GameId: gameID}
	jsonData, err := protojson.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/v1/games/"+gameID+"/start", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		v := make(map[string]string)
		_ = json.Unmarshal(body, &v)
		if v["message"] == "game already started" {
			return nil // good!
		}
		return fmt.Errorf("start game failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) GetState(gameID string) (*api.State, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/v1/games/"+gameID+"/state", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get state failed with status %d: %s", resp.StatusCode, string(body))
	}

	var stateResp api.GetStateResponse
	if err := protojson.Unmarshal(body, &stateResp); err != nil {
		return nil, err
	}

	return stateResp.State, nil
}

func (c *Client) PostAction(action *api.PostActionRequest) error {
	jsonData, err := protojson.Marshal(action)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/v1/games/"+action.GameId+"/action", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("post action failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
