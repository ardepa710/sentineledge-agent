package vault

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
)

type VaultClient struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	httpClient   *http.Client
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

type syncResponse struct {
	Ciphers []cipher `json:"ciphers"`
}

type cipher struct {
	Type  int    `json:"type"`
	Name  string `json:"name"`
	Login *login `json:"login"`
	Data  *data  `json:"data"`
}

type login struct {
	Password string `json:"password"`
}

type data struct {
	Password string `json:"password"`
}

func NewClient(baseURL, clientID, clientSecret string) *VaultClient {
	return &VaultClient{
		BaseURL:      baseURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (v *VaultClient) getToken() (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", v.ClientID)
	form.Set("client_secret", v.ClientSecret)
	form.Set("scope", "api")
	form.Set("device_type", "21")
	form.Set("device_identifier", uuid.New().String())
	form.Set("device_name", "sentineledge-agent")

	resp, err := v.httpClient.Post(
		v.BaseURL+"/identity/connect/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("vault token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("vault token error %d: %s", resp.StatusCode, string(body))
	}

	var t tokenResponse
	if err := json.Unmarshal(body, &t); err != nil {
		return "", fmt.Errorf("vault token parse error: %w", err)
	}

	return t.AccessToken, nil
}

// GetSecret obtiene un secreto por nombre desde Vaultwarden
func (v *VaultClient) GetSecret(name string) (string, error) {
	token, err := v.getToken()
	if err != nil {
		return "", err
	}

	req, _ := http.NewRequest("GET", v.BaseURL+"/api/sync", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault sync failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var sync syncResponse
	if err := json.Unmarshal(body, &sync); err != nil {
		return "", fmt.Errorf("vault sync parse error: %w", err)
	}

	for _, c := range sync.Ciphers {
		if c.Type != 1 || c.Name != name {
			continue
		}
		if c.Login != nil && c.Login.Password != "" {
			return c.Login.Password, nil
		}
		if c.Data != nil && c.Data.Password != "" {
			return c.Data.Password, nil
		}
	}

	return "", fmt.Errorf("secret '%s' not found in vault", name)
}

func (v *VaultClient) StoreSecret(name, value, orgID, collectionID string) error {
	token, err := v.getToken()
	if err != nil {
		return err
	}

	// Paso 1: Crear el cipher
	body := map[string]interface{}{
		"organizationId": orgID,
		"collectionIds":  []string{collectionID},
		"type":           1,
		"name":           name,
		"login": map[string]interface{}{
			"username": name,
			"password": value,
			"uris":     []string{},
		},
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", v.BaseURL+"/api/ciphers", strings.NewReader(string(jsonBody)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault store failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("vault store error %d: %s", resp.StatusCode, string(respBody))
	}

	// Obtener ID del cipher creado
	var created struct {
		ID string `json:"id"`
	}
	json.Unmarshal(respBody, &created)

	if created.ID == "" {
		return fmt.Errorf("could not get cipher ID from response")
	}

	// Paso 2: Mover a la colección correcta
	moveBody := map[string]interface{}{
		"collectionIds": []string{collectionID},
	}
	moveJSON, _ := json.Marshal(moveBody)

	moveReq, _ := http.NewRequest(
		"PUT",
		v.BaseURL+"/api/ciphers/"+created.ID+"/collections",
		strings.NewReader(string(moveJSON)),
	)
	moveReq.Header.Set("Authorization", "Bearer "+token)
	moveReq.Header.Set("Content-Type", "application/json")

	moveResp, err := v.httpClient.Do(moveReq)
	if err != nil {
		return fmt.Errorf("vault move to collection failed: %w", err)
	}
	defer moveResp.Body.Close()

	if moveResp.StatusCode != 200 {
		moveBody, _ := io.ReadAll(moveResp.Body)
		return fmt.Errorf("vault move error %d: %s", moveResp.StatusCode, string(moveBody))
	}

	return nil
}

// StoreSecretViaCLI guarda un secreto usando el Bitwarden CLI (visible en GUI)
func (v *VaultClient) StoreSecretViaCLI(name, value, orgID, collectionID string) error {
	template := map[string]interface{}{
		"organizationId": orgID,
		"collectionIds":  []string{collectionID},
		"type":           1,
		"name":           name,
		"login": map[string]interface{}{
			"username": name,
			"password": value,
			"uris":     []string{},
		},
	}

	jsonBytes, err := json.Marshal(template)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(jsonBytes)

	// Usar BW_SESSION del entorno si está disponible
	bwSession := os.Getenv("BW_SESSION")

	var cmd *exec.Cmd
	if bwSession != "" {
		cmd = exec.Command("bw", "create", "item", "--session", bwSession, encoded)
	} else {
		cmd = exec.Command("bw", "create", "item", encoded)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bw create failed: %w — %s", err, string(output))
	}

	return nil
}
