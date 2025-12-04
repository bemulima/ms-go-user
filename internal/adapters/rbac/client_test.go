package rbac

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPClient_AssignRole(t *testing.T) {
	t.Parallel()

	var payload map[string]interface{}
	client := &httpClient{
		baseURL: "http://rbac-service",
		client: newMockHTTPClient(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, "/assign_role", req.URL.Path)
			require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
	}

	require.NoError(t, client.AssignRole(context.Background(), "user-123", "role-admin"))

	value, ok := payload["value"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "user-123", value["user_id"])
	require.Equal(t, "role-admin", value["role"])
}

func TestHTTPClient_GetRoleByUserID(t *testing.T) {
	t.Parallel()

	const expectedRole = "role-captain"
	client := &httpClient{
		baseURL: "http://rbac-service",
		client: newMockHTTPClient(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/get_role_by_user_id", req.URL.Path)
			require.Equal(t, "user-789", req.URL.Query().Get("user_id"))
			body := `{"role":"` + expectedRole + `"}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		}),
	}

	role, err := client.GetRoleByUserID(context.Background(), "user-789")
	require.NoError(t, err)
	require.Equal(t, expectedRole, role)
}

type mockRoundTripper func(*http.Request) (*http.Response, error)

func (m mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func newMockHTTPClient(tripper mockRoundTripper) *http.Client {
	return &http.Client{Transport: tripper}
}
