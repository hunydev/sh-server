package srv

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/hunydev/sh-server/db/dbgen"
)

// Script represents a script in API responses
type ScriptResponse struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Content     string    `json:"content"`
	Description string    `json:"description"`
	Tags        string    `json:"tags"`
	Locked      bool      `json:"locked"`
	DangerLevel int       `json:"danger_level"`
	Requires    string    `json:"requires"`
	Examples    string    `json:"examples"`
	Favorite    bool      `json:"favorite"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func scriptToResponse(s dbgen.Script) ScriptResponse {
	resp := ScriptResponse{
		ID:        s.ID,
		Path:      s.Path,
		Name:      s.Name,
		Content:   s.Content,
		Locked:    s.Locked != 0,
		Favorite:  s.Favorite != 0,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
	if s.Description != nil {
		resp.Description = *s.Description
	}
	if s.Tags != nil {
		resp.Tags = *s.Tags
	}
	if s.DangerLevel != nil {
		resp.DangerLevel = int(*s.DangerLevel)
	}
	if s.Requires != nil {
		resp.Requires = *s.Requires
	}
	if s.Examples != nil {
		resp.Examples = *s.Examples
	}
	return resp
}

// APIListScripts returns all scripts
func (s *Server) APIListScripts(w http.ResponseWriter, r *http.Request) {
	q := dbgen.New(s.DB)
	scripts, err := q.ListScripts(r.Context())
	if err != nil {
		http.Error(w, "Failed to list scripts", http.StatusInternalServerError)
		return
	}
	
	resp := make([]ScriptResponse, len(scripts))
	for i, sc := range scripts {
		resp[i] = scriptToResponse(sc)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// APIGetScript returns a single script by ID
func (s *Server) APIGetScript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	
	q := dbgen.New(s.DB)
	script, err := q.GetScript(r.Context(), id)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scriptToResponse(script))
}

// CreateScriptRequest represents a request to create a script
type CreateScriptRequest struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Locked      bool   `json:"locked"`
	Password    string `json:"password,omitempty"`
	DangerLevel int    `json:"danger_level"`
	Requires    string `json:"requires"`
	Examples    string `json:"examples"`
}

// APICreateScript creates a new script
func (s *Server) APICreateScript(w http.ResponseWriter, r *http.Request) {
	var req CreateScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if err := validatePath(req.Path); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// Hash password if locked
	var passwordHash *string
	if req.Locked && req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}
		hashStr := string(hash)
		passwordHash = &hashStr
	}
	
	now := time.Now()
	id := uuid.New().String()
	name := extractName(req.Path)
	
	lockedInt := int64(0)
	if req.Locked {
		lockedInt = 1
	}
	dangerLevel := int64(req.DangerLevel)
	
	q := dbgen.New(s.DB)
	
	// Ensure parent folders exist
	s.ensureFolders(r.Context(), q, req.Path)
	
	err := q.CreateScript(r.Context(), dbgen.CreateScriptParams{
		ID:           id,
		Path:         req.Path,
		Name:         name,
		Content:      req.Content,
		Description:  &req.Description,
		Tags:         &req.Tags,
		Locked:       lockedInt,
		PasswordHash: passwordHash,
		DangerLevel:  &dangerLevel,
		Requires:     &req.Requires,
		Examples:     &req.Examples,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			http.Error(w, "Script with this path already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to create script: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Create initial version
	q.CreateVersion(r.Context(), dbgen.CreateVersionParams{
		ScriptID:  id,
		Content:   req.Content,
		Version:   1,
		CreatedAt: now,
	})
	
	// Log creation
	q.CreateAuditLog(r.Context(), dbgen.CreateAuditLogParams{
		Action:     "CREATE",
		EntityType: "script",
		EntityID:   &id,
		EntityPath: &req.Path,
		CreatedAt:  now,
	})
	
	script, _ := q.GetScript(r.Context(), id)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(scriptToResponse(script))
}

// UpdateScriptRequest represents a request to update a script
type UpdateScriptRequest struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Locked      bool   `json:"locked"`
	Password    string `json:"password,omitempty"`
	DangerLevel int    `json:"danger_level"`
	Requires    string `json:"requires"`
	Examples    string `json:"examples"`
}

// APIUpdateScript updates an existing script
func (s *Server) APIUpdateScript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	
	var req UpdateScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if err := validatePath(req.Path); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	q := dbgen.New(s.DB)
	
	// Get existing script
	existing, err := q.GetScript(r.Context(), id)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}
	
	// Hash password if locked and password provided
	var passwordHash *string
	if req.Locked {
		if req.Password != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			if err != nil {
				http.Error(w, "Failed to hash password", http.StatusInternalServerError)
				return
			}
			hashStr := string(hash)
			passwordHash = &hashStr
		} else {
			// Keep existing password hash
			passwordHash = existing.PasswordHash
		}
	}
	
	now := time.Now()
	name := extractName(req.Path)
	
	lockedInt := int64(0)
	if req.Locked {
		lockedInt = 1
	}
	dangerLevel := int64(req.DangerLevel)
	
	err = q.UpdateScript(r.Context(), dbgen.UpdateScriptParams{
		Path:         req.Path,
		Name:         name,
		Content:      req.Content,
		Description:  &req.Description,
		Tags:         &req.Tags,
		Locked:       lockedInt,
		PasswordHash: passwordHash,
		DangerLevel:  &dangerLevel,
		Requires:     &req.Requires,
		Examples:     &req.Examples,
		UpdatedAt:    now,
		ID:           id,
	})
	
	if err != nil {
		http.Error(w, "Failed to update script: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Create new version if content changed
	if existing.Content != req.Content {
		versions, _ := q.ListVersions(r.Context(), id)
		newVersion := int64(1)
		if len(versions) > 0 {
			newVersion = versions[0].Version + 1
		}
		q.CreateVersion(r.Context(), dbgen.CreateVersionParams{
			ScriptID:  id,
			Content:   req.Content,
			Version:   newVersion,
			CreatedAt: now,
		})
	}
	
	// Log update
	q.CreateAuditLog(r.Context(), dbgen.CreateAuditLogParams{
		Action:     "UPDATE",
		EntityType: "script",
		EntityID:   &id,
		EntityPath: &req.Path,
		CreatedAt:  now,
	})
	
	script, _ := q.GetScript(r.Context(), id)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scriptToResponse(script))
}

// APIDeleteScript deletes a script
func (s *Server) APIDeleteScript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	
	q := dbgen.New(s.DB)
	
	script, err := q.GetScript(r.Context(), id)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}
	
	if err := q.DeleteScript(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete script", http.StatusInternalServerError)
		return
	}
	
	// Log deletion
	q.CreateAuditLog(r.Context(), dbgen.CreateAuditLogParams{
		Action:     "DELETE",
		EntityType: "script",
		EntityID:   &id,
		EntityPath: &script.Path,
		CreatedAt:  time.Now(),
	})
	
	w.WriteHeader(http.StatusNoContent)
}

// TreeNode represents a node in the folder tree
type TreeNode struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"` // "folder" or "script"
	Locked   bool        `json:"locked,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`
}

// APIGetTree returns the folder/script tree
func (s *Server) APIGetTree(w http.ResponseWriter, r *http.Request) {
	q := dbgen.New(s.DB)
	scripts, _ := q.ListScripts(r.Context())
	folders, _ := q.ListFolders(r.Context())
	
	// Build tree from paths
	root := &TreeNode{
		ID:       "root",
		Name:     "/",
		Path:     "/",
		Type:     "folder",
		Children: []*TreeNode{},
	}
	
	// Map for quick lookup
	nodeMap := map[string]*TreeNode{"/": root}
	
	// Add folders
	for _, f := range folders {
		node := &TreeNode{
			ID:       f.ID,
			Name:     f.Name,
			Path:     f.Path,
			Type:     "folder",
			Children: []*TreeNode{},
		}
		nodeMap[f.Path] = node
	}
	
	// Add scripts
	for _, sc := range scripts {
		node := &TreeNode{
			ID:     sc.ID,
			Name:   sc.Name,
			Path:   sc.Path,
			Type:   "script",
			Locked: sc.Locked != 0,
		}
		nodeMap[sc.Path] = node
	}
	
	// Build hierarchy
	for path, node := range nodeMap {
		if path == "/" {
			continue
		}
		
		parentPath := getParentPath(path)
		if parent, ok := nodeMap[parentPath]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			root.Children = append(root.Children, node)
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(root)
}

func getParentPath(path string) string {
	if path == "/" {
		return "/"
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) <= 1 {
		return "/"
	}
	return "/" + strings.Join(parts[:len(parts)-1], "/")
}

// FolderResponse represents a folder in API responses
type FolderResponse struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// APIListFolders returns all folders
func (s *Server) APIListFolders(w http.ResponseWriter, r *http.Request) {
	q := dbgen.New(s.DB)
	folders, err := q.ListFolders(r.Context())
	if err != nil {
		http.Error(w, "Failed to list folders", http.StatusInternalServerError)
		return
	}
	
	resp := make([]FolderResponse, len(folders))
	for i, f := range folders {
		resp[i] = FolderResponse{
			ID:        f.ID,
			Path:      f.Path,
			Name:      f.Name,
			CreatedAt: f.CreatedAt,
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// CreateFolderRequest represents a request to create a folder
type CreateFolderRequest struct {
	Path string `json:"path"`
}

// APICreateFolder creates a new folder
func (s *Server) APICreateFolder(w http.ResponseWriter, r *http.Request) {
	var req CreateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if !strings.HasPrefix(req.Path, "/") {
		http.Error(w, "Path must start with /", http.StatusBadRequest)
		return
	}
	
	q := dbgen.New(s.DB)
	s.ensureFolders(r.Context(), q, req.Path+"/dummy.sh")
	
	// Get the created folder
	folder, err := q.GetFolderByPath(r.Context(), req.Path)
	if err != nil {
		http.Error(w, "Failed to create folder", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(FolderResponse{
		ID:        folder.ID,
		Path:      folder.Path,
		Name:      folder.Name,
		CreatedAt: folder.CreatedAt,
	})
}

// APIDeleteFolder deletes a folder
func (s *Server) APIDeleteFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	
	q := dbgen.New(s.DB)
	
	folder, err := q.GetFolder(r.Context(), id)
	if err != nil {
		http.Error(w, "Folder not found", http.StatusNotFound)
		return
	}
	
	if err := q.DeleteFolderByPath(r.Context(), dbgen.DeleteFolderByPathParams{
		Path:    folder.Path,
		Column2: &folder.Path,
	}); err != nil {
		http.Error(w, "Failed to delete folder", http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// APISearch searches scripts
func (s *Server) APISearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}
	
	q := dbgen.New(s.DB)
	scripts, err := q.SearchScripts(r.Context(), dbgen.SearchScriptsParams{
		Column1: &query,
		Column2: &query,
		Column3: &query,
		Column4: &query,
	})
	if err != nil {
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}
	
	resp := make([]ScriptResponse, len(scripts))
	for i, sc := range scripts {
		resp[i] = scriptToResponse(sc)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ensureFolders creates all parent folders for a given script path
func (s *Server) ensureFolders(ctx context.Context, q *dbgen.Queries, scriptPath string) {
	parts := strings.Split(strings.TrimPrefix(scriptPath, "/"), "/")
	if len(parts) <= 1 {
		return // No parent folders needed
	}
	
	// Create all parent folders
	for i := 1; i < len(parts); i++ {
		folderPath := "/" + strings.Join(parts[:i], "/")
		folderName := parts[i-1]
		
		// Check if folder exists
		_, err := q.GetFolderByPath(ctx, folderPath)
		if err != nil {
			// Create folder
			q.CreateFolder(ctx, dbgen.CreateFolderParams{
				ID:        uuid.New().String(),
				Path:      folderPath,
				Name:      folderName,
				CreatedAt: time.Now(),
			})
		}
	}
}
