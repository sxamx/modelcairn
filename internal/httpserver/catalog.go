package httpserver

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/sxamx/modelcairn/internal/storage"
)

type pageCursor struct {
	Version int    `json:"v"`
	Scope   string `json:"scope"`
	ID      string `json:"id"`
}

var resourceKinds = map[string]storage.ResourceKind{
	"providers": storage.KindProvider, "provider-accounts": storage.KindProviderAccount,
	"provider-connections": storage.KindProviderConnection, "credentials": storage.KindCredential,
	"egresses": storage.KindEgress, "models": storage.KindModel, "destinations": storage.KindDestination,
	"strategies": storage.KindStrategy, "routes": storage.KindRoute, "agent-tokens": storage.KindAgentToken,
}

func (a *adminAPI) authorizeRead(w http.ResponseWriter, r *http.Request) bool {
	session, csrf, status := a.authorize(r, false)
	if status != 0 {
		writeAdminError(w, status, boundaryCode(status), false)
		return false
	}
	if _, err := storage.UseAdminSession(r.Context(), a.installation, session, csrf, true, time.Now()); err != nil {
		writeSessionError(w, err)
		return false
	}
	return true
}

func (a *adminAPI) listResources(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRead(w, r) {
		return
	}
	scope := r.PathValue("kind")
	kind, ok := resourceKinds[scope]
	if !ok {
		writeAdminError(w, http.StatusBadRequest, "invalid_resource_kind", false)
		return
	}
	limit, after, err := pageInput(r, "resources:"+scope)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_page", false)
		return
	}
	page, err := a.repository.ListPage(r.Context(), kind, after, limit)
	if err != nil {
		writeAdminError(w, http.StatusServiceUnavailable, "unavailable", true)
		return
	}
	items := make([]any, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, resourceView(item))
	}
	var next any
	if page.NextID != "" {
		next = encodeCursor("resources:"+scope, page.NextID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": next})
}

func (a *adminAPI) getResource(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRead(w, r) {
		return
	}
	kind, ok := resourceKinds[r.PathValue("kind")]
	if !ok {
		writeAdminError(w, http.StatusBadRequest, "invalid_resource_kind", false)
		return
	}
	item, err := a.repository.Get(r.Context(), kind, r.PathValue("name"))
	if err != nil {
		writeReadError(w, err)
		return
	}
	w.Header().Set("ETag", quotedVersion(item.ResourceVersion))
	writeJSON(w, http.StatusOK, resourceView(item))
}

func (a *adminAPI) listSecrets(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRead(w, r) {
		return
	}
	limit, after, err := pageInput(r, "secrets")
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_page", false)
		return
	}
	page, err := a.installation.Secrets().ListMetadataPage(r.Context(), after, limit)
	if err != nil {
		writeAdminError(w, http.StatusServiceUnavailable, "unavailable", true)
		return
	}
	var next any
	if page.NextID != "" {
		next = encodeCursor("secrets", page.NextID)
	}
	items := make([]any, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, secretMetadataView(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": next})
}

func (a *adminAPI) getSecret(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeRead(w, r) {
		return
	}
	metadata, err := a.installation.Secrets().GetMetadata(r.Context(), r.PathValue("name"))
	if err != nil {
		writeReadError(w, err)
		return
	}
	w.Header().Set("ETag", quotedVersion(metadata.ResourceVersion))
	writeJSON(w, http.StatusOK, secretMetadataView(metadata))
}

func secretMetadataView(item storage.SecretMetadata) map[string]any {
	return map[string]any{"name": item.Name, "fingerprint": item.Fingerprint, "resourceVersion": item.ResourceVersion, "updatedAt": item.UpdatedAt}
}

func resourceView(item storage.Resource) map[string]any {
	metadata := map[string]any{"name": item.Name, "uid": item.ID, "resourceVersion": item.ResourceVersion}
	if item.DisplayName != nil {
		metadata["displayName"] = *item.DisplayName
	}
	if item.Description != nil {
		metadata["description"] = *item.Description
	}
	return map[string]any{"kind": item.Kind, "state": "present", "metadata": metadata, "spec": json.RawMessage(item.Spec)}
}
func quotedVersion(version int64) string { return `"` + strconv.FormatInt(version, 10) + `"` }
func writeReadError(w http.ResponseWriter, err error) {
	if storage.IsRepositoryCode(err, storage.CodeNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", false)
		return
	}
	if storage.IsRepositoryCode(err, storage.CodeInvalidResource) {
		writeAdminError(w, http.StatusBadRequest, "invalid_resource", false)
		return
	}
	writeAdminError(w, http.StatusServiceUnavailable, "unavailable", true)
}
func pageInput(r *http.Request, scope string) (int, string, error) {
	query := r.URL.Query()
	limit := 50
	if values, ok := query["limit"]; ok {
		if len(values) != 1 {
			return 0, "", errors.New("invalid_page")
		}
		parsed, err := strconv.Atoi(values[0])
		if err != nil || parsed < 1 || parsed > 200 {
			return 0, "", errors.New("invalid_page")
		}
		limit = parsed
	}
	after := ""
	if values, ok := query["cursor"]; ok {
		if len(values) != 1 || len(values[0]) > 512 {
			return 0, "", errors.New("invalid_page")
		}
		var err error
		after, err = decodeCursor(values[0], scope)
		if err != nil {
			return 0, "", err
		}
	}
	return limit, after, nil
}
func encodeCursor(scope, id string) string {
	data, _ := json.Marshal(pageCursor{1, scope, id})
	return base64.RawURLEncoding.EncodeToString(data)
}
func decodeCursor(value, scope string) (string, error) {
	data, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(data) > 256 {
		return "", errors.New("invalid_page")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cursor pageCursor
	if decoder.Decode(&cursor) != nil || decoder.Decode(new(any)) != io.EOF || cursor.Version != 1 || cursor.Scope != scope || cursor.ID == "" || len(cursor.ID) > 64 {
		return "", errors.New("invalid_page")
	}
	return cursor.ID, nil
}
