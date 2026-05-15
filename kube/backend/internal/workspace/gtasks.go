package workspace

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Google Tasks adapter — list-CRUD against tasks v1.

const gtasksAPIBase = "https://tasks.googleapis.com/tasks/v1"

type TaskList struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type Task struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Notes   string `json:"notes,omitempty"`
	Status  string `json:"status"` // "needsAction" | "completed"
	Due     string `json:"due,omitempty"`
	Updated string `json:"updated,omitempty"`
}

type TaskManager interface {
	Integration
	ListTaskLists(ctx context.Context, token Token) ([]TaskList, error)
	ListTasks(ctx context.Context, token Token, listID string) ([]Task, error)
	CreateTask(ctx context.Context, token Token, listID string, task Task) (Task, error)
	UpdateTask(ctx context.Context, token Token, listID, taskID string, patch Task) (Task, error)
	DeleteTask(ctx context.Context, token Token, listID, taskID string) error
}

type GTasksAdapter struct {
	HTTPClient *http.Client
	APIBase    string
}

func NewGTasksAdapter() *GTasksAdapter { return &GTasksAdapter{} }

func (a *GTasksAdapter) Service() Service { return ServiceGoogle }

func (a *GTasksAdapter) base() string {
	if a.APIBase != "" {
		return a.APIBase
	}
	return gtasksAPIBase
}

type gtasksListsResp struct {
	Items []TaskList `json:"items"`
}

func (a *GTasksAdapter) ListTaskLists(ctx context.Context, token Token) ([]TaskList, error) {
	hc := googleClient(a.HTTPClient)
	var out gtasksListsResp
	if err := googleAPICall(ctx, hc, token, http.MethodGet, a.base()+"/users/@me/lists", nil, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

type gtasksTasksResp struct {
	Items []Task `json:"items"`
}

func (a *GTasksAdapter) ListTasks(ctx context.Context, token Token, listID string) ([]Task, error) {
	if strings.TrimSpace(listID) == "" {
		return nil, fmt.Errorf("gtasks: listID required")
	}
	hc := googleClient(a.HTTPClient)
	endpoint := a.base() + "/lists/" + url.PathEscape(listID) + "/tasks?maxResults=100&showCompleted=true&showHidden=false"
	var out gtasksTasksResp
	if err := googleAPICall(ctx, hc, token, http.MethodGet, endpoint, nil, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

func (a *GTasksAdapter) CreateTask(ctx context.Context, token Token, listID string, t Task) (Task, error) {
	if strings.TrimSpace(listID) == "" {
		return Task{}, fmt.Errorf("gtasks: listID required")
	}
	if strings.TrimSpace(t.Title) == "" {
		return Task{}, fmt.Errorf("gtasks: title required")
	}
	if t.Status == "" {
		t.Status = "needsAction"
	}
	hc := googleClient(a.HTTPClient)
	endpoint := a.base() + "/lists/" + url.PathEscape(listID) + "/tasks"
	var out Task
	if err := googleAPICall(ctx, hc, token, http.MethodPost, endpoint, t, &out); err != nil {
		return Task{}, err
	}
	return out, nil
}

// UpdateTask uses PATCH so callers can flip a single field (status,
// title, due) without resending the whole object.
func (a *GTasksAdapter) UpdateTask(ctx context.Context, token Token, listID, taskID string, patch Task) (Task, error) {
	if strings.TrimSpace(listID) == "" || strings.TrimSpace(taskID) == "" {
		return Task{}, fmt.Errorf("gtasks: listID + taskID required")
	}
	hc := googleClient(a.HTTPClient)
	endpoint := a.base() + "/lists/" + url.PathEscape(listID) + "/tasks/" + url.PathEscape(taskID)
	patch.ID = taskID
	var out Task
	if err := googleAPICall(ctx, hc, token, http.MethodPatch, endpoint, patch, &out); err != nil {
		return Task{}, err
	}
	return out, nil
}

func (a *GTasksAdapter) DeleteTask(ctx context.Context, token Token, listID, taskID string) error {
	if strings.TrimSpace(listID) == "" || strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("gtasks: listID + taskID required")
	}
	hc := googleClient(a.HTTPClient)
	endpoint := a.base() + "/lists/" + url.PathEscape(listID) + "/tasks/" + url.PathEscape(taskID)
	return googleAPICall(ctx, hc, token, http.MethodDelete, endpoint, nil, nil)
}
