package srv

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/hunydev/sh-server/db"
	"github.com/hunydev/sh-server/db/dbgen"
)

//go:embed static/*
var staticFS embed.FS

//go:embed templates/*
var templatesFS embed.FS

type Server struct {
	DB         *sql.DB
	Hostname   string
	AdminToken string
}

type Config struct {
	DBPath     string
	Hostname   string
	AdminToken string
}

func New(cfg Config) (*Server, error) {
	srv := &Server{
		Hostname:   cfg.Hostname,
		AdminToken: cfg.AdminToken,
	}
	if err := srv.setUpDatabase(cfg.DBPath); err != nil {
		return nil, err
	}
	return srv, nil
}

func (s *Server) setUpDatabase(dbPath string) error {
	wdb, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}
	s.DB = wdb
	if err := db.RunMigrations(wdb); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// isCLI checks if the request is from a CLI tool (curl, wget, etc)
func isCLI(r *http.Request) bool {
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	cliPatterns := []string{"curl", "wget", "httpie", "fetch", "libfetch", "aria2", "python-requests", "go-http-client"}
	for _, p := range cliPatterns {
		if strings.Contains(ua, p) {
			return true
		}
	}
	
	// Also check Accept header - browsers prefer text/html
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		return false
	}
	
	// If no User-Agent and not asking for HTML, assume CLI
	if ua == "" && !strings.Contains(accept, "text/html") {
		return true
	}
	
	return false
}

// HandleRoot handles the root path with content negotiation
func (s *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	if isCLI(r) {
		// CLI response: 2 lines
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "curl -fsSL https://%s/help.sh | sh\n", s.Hostname)
		fmt.Fprintf(w, "curl -fsSL https://%s/search.sh | sh\n", s.Hostname)
		return
	}
	
	// Browser response: serve HTML
	s.serveHTML(w, r)
}

func (s *Server) serveHTML(w http.ResponseWriter, r *http.Request) {
	data, err := templatesFS.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// HandleHelp serves the help.sh script
func (s *Server) HandleHelp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=300")
	fmt.Fprintf(w, `#!/bin/sh
# sh.huny.dev - Script Repository
# ================================

cat << 'EOF'

sh.huny.dev - Personal Script Repository
=========================================

Usage:
  curl -fsSL https://%s/help.sh | sh        # Show this help
  curl -fsSL https://%s/search.sh | sh      # Interactive search (TUI)
  curl -fsSL https://%s/<path>.sh | sh      # Run a specific script

Examples:
  curl -fsSL https://%s/tools/sysinfo.sh | sh
  curl -fsSL https://%s/network/check.sh | sh

Browse scripts at: https://%s

EOF
`, s.Hostname, s.Hostname, s.Hostname, s.Hostname, s.Hostname, s.Hostname)
}

// HandleSearch serves the search.sh TUI script
func (s *Server) HandleSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	
	script := fmt.Sprintf(`#!/bin/sh
# Interactive script browser for sh.huny.dev
# Hierarchical folder navigation

BASE_URL="https://%s"
CURRENT_PATH="/"

# Fetch catalog
fetch_catalog() {
    curl -fsSL "${BASE_URL}/_catalog.json" 2>/dev/null
}

# Check for available TUI tools
has_cmd() {
    command -v "$1" >/dev/null 2>&1
}

# Get all script paths from catalog
get_all_paths() {
    echo "$CATALOG" | sed 's/},{/}\n{/g' | grep -o '"path":"[^"]*"' | sed 's/"path":"\([^"]*\)"/\1/' | sort
}

# Get items (folders and scripts) in current path
# Returns: folder names (with /) and script names for current directory only
get_current_items() {
    _cur_path="$1"
    _all_paths=$(get_all_paths)
    
    # Normalize current path
    if [ "$_cur_path" = "/" ]; then
        _prefix=""
    else
        _prefix="$_cur_path"
    fi
    
    _folders=""
    _scripts=""
    
    for _path in $_all_paths; do
        # Check if path starts with current prefix
        case "$_path" in
            ${_prefix}/*)
                # Get the remainder after prefix
                _remainder="${_path#${_prefix}/}"
                
                # Check if it's a direct child or nested
                case "$_remainder" in
                    */*)
                        # Has more slashes - extract first folder
                        _folder=$(echo "$_remainder" | cut -d'/' -f1)
                        # Add to folders if not already added
                        case " $_folders " in
                            *" $_folder "*) ;;
                            *) _folders="$_folders $_folder" ;;
                        esac
                        ;;
                    *)
                        # Direct child script
                        _scripts="$_scripts $_remainder"
                        ;;
                esac
                ;;
        esac
    done
    
    # Output folders first (with / suffix), then scripts
    for _f in $_folders; do
        [ -n "$_f" ] && echo "📁 $_f/"
    done
    for _s in $_scripts; do
        [ -n "$_s" ] && echo "📄 $_s"
    done
}

# FZF-based hierarchical browser
browse_fzf() {
    while true; do
        ITEMS=$(get_current_items "$CURRENT_PATH")
        
        if [ -z "$ITEMS" ] && [ "$CURRENT_PATH" = "/" ]; then
            echo "No scripts found"
            exit 0
        fi
        
        # Build menu with navigation options
        MENU=""
        if [ "$CURRENT_PATH" != "/" ]; then
            MENU="⬆️  ../"
        fi
        if [ -n "$ITEMS" ]; then
            if [ -n "$MENU" ]; then
                MENU=$(printf "%%s\n%%s" "$MENU" "$ITEMS")
            else
                MENU="$ITEMS"
            fi
        fi
        
        # Show current path in prompt
        SELECTED=$(echo "$MENU" | fzf --height=50%% --border \
            --header="📂 $CURRENT_PATH" \
            --prompt="Select: " \
            --preview='
                item="{}"
                base_url="'"${BASE_URL}"'"
                cur="'"${CURRENT_PATH}"'"
                if echo "$item" | grep -q "^📁"; then
                    echo "📁 Folder - press Enter to navigate"
                elif echo "$item" | grep -q "^\\.\\./\\|^⬆️"; then
                    echo "⬆️  Go to parent folder"
                else
                    name=$(echo "$item" | sed "s/^📄 //")
                    if [ "$cur" = "/" ]; then
                        path="/${name}"
                    else
                        path="${cur}/${name}"
                    fi
                    curl -fsSL "${base_url}${path}?preview=1" 2>/dev/null || echo "Preview not available"
                fi
            ')
        
        [ -z "$SELECTED" ] && exit 0
        
        # Handle selection
        case "$SELECTED" in
            "⬆️  ../")
                # Go up one level
                CURRENT_PATH=$(dirname "$CURRENT_PATH")
                [ "$CURRENT_PATH" = "." ] && CURRENT_PATH="/"
                ;;
            "📁 "*)
                # Enter folder
                FOLDER=$(echo "$SELECTED" | sed 's/^📁 //' | sed 's/\/$//')
                if [ "$CURRENT_PATH" = "/" ]; then
                    CURRENT_PATH="/$FOLDER"
                else
                    CURRENT_PATH="$CURRENT_PATH/$FOLDER"
                fi
                ;;
            "📄 "*)
                # Run script
                SCRIPT=$(echo "$SELECTED" | sed 's/^📄 //')
                if [ "$CURRENT_PATH" = "/" ]; then
                    SCRIPT_PATH="/$SCRIPT"
                else
                    SCRIPT_PATH="$CURRENT_PATH/$SCRIPT"
                fi
                echo ""
                echo "Running: ${BASE_URL}${SCRIPT_PATH}"
                echo ""
                curl -fsSL "${BASE_URL}${SCRIPT_PATH}" | sh
                exit 0
                ;;
        esac
    done
}

# Dialog/Whiptail hierarchical browser
browse_dialog() {
    DIALOG_CMD="$1"
    
    while true; do
        ITEMS=$(get_current_items "$CURRENT_PATH")
        
        if [ -z "$ITEMS" ] && [ "$CURRENT_PATH" = "/" ]; then
            echo "No scripts found"
            exit 0
        fi
        
        # Build menu items
        MENU_ITEMS=""
        i=1
        
        # Add parent navigation if not at root
        if [ "$CURRENT_PATH" != "/" ]; then
            MENU_ITEMS="$MENU_ITEMS 0 \"⬆️  ../  (parent folder)\""
        fi
        
        # Add folders and scripts
        echo "$ITEMS" | while read -r item; do
            [ -z "$item" ] && continue
            echo "$i $item"
            i=$((i+1))
        done > /tmp/sh_menu_$$
        
        while read -r num item; do
            MENU_ITEMS="$MENU_ITEMS $num \"$item\""
        done < /tmp/sh_menu_$$
        rm -f /tmp/sh_menu_$$
        
        ITEM_COUNT=$(echo "$ITEMS" | grep -c . || echo 0)
        
        CHOICE=$(eval "$DIALOG_CMD --title 'sh.huny.dev' --menu 'Current: $CURRENT_PATH' 20 60 15 $MENU_ITEMS" 3>&1 1>&2 2>&3 || true)
        
        [ -z "$CHOICE" ] && exit 0
        
        if [ "$CHOICE" = "0" ]; then
            # Go up
            CURRENT_PATH=$(dirname "$CURRENT_PATH")
            [ "$CURRENT_PATH" = "." ] && CURRENT_PATH="/"
            continue
        fi
        
        # Get selected item
        SELECTED=$(echo "$ITEMS" | sed -n "${CHOICE}p")
        
        case "$SELECTED" in
            "📁 "*)
                # Enter folder
                FOLDER=$(echo "$SELECTED" | sed 's/^📁 //' | sed 's/\/$//')
                if [ "$CURRENT_PATH" = "/" ]; then
                    CURRENT_PATH="/$FOLDER"
                else
                    CURRENT_PATH="$CURRENT_PATH/$FOLDER"
                fi
                ;;
            "📄 "*)
                # Run script
                SCRIPT=$(echo "$SELECTED" | sed 's/^📄 //')
                if [ "$CURRENT_PATH" = "/" ]; then
                    SCRIPT_PATH="/$SCRIPT"
                else
                    SCRIPT_PATH="$CURRENT_PATH/$SCRIPT"
                fi
                clear
                echo "Running: ${BASE_URL}${SCRIPT_PATH}"
                echo ""
                curl -fsSL "${BASE_URL}${SCRIPT_PATH}" | sh
                exit 0
                ;;
        esac
    done
}

# Fallback: number-based hierarchical menu
browse_fallback() {
    while true; do
        ITEMS=$(get_current_items "$CURRENT_PATH")
        
        if [ -z "$ITEMS" ] && [ "$CURRENT_PATH" = "/" ]; then
            echo "No scripts found"
            exit 0
        fi
        
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "  sh.huny.dev - Script Browser"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "  📂 Current: $CURRENT_PATH"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""
        
        i=1
        
        # Show parent option if not at root
        if [ "$CURRENT_PATH" != "/" ]; then
            echo "  ..) ⬆️  ../ (parent folder)"
            echo ""
        fi
        
        # Show items
        echo "$ITEMS" | while read -r item; do
            [ -z "$item" ] && continue
            printf "  %%2d) %%s\n" "$i" "$item"
            i=$((i+1))
        done
        
        ITEM_COUNT=$(echo "$ITEMS" | grep -c . || echo 0)
        
        echo ""
        echo "   0) Exit"
        echo ""
        printf "Select [0-%d or ..]: " "$ITEM_COUNT"
        read -r CHOICE
        
        # Handle exit
        if [ "$CHOICE" = "0" ] || [ -z "$CHOICE" ]; then
            echo "Goodbye!"
            exit 0
        fi
        
        # Handle parent navigation
        if [ "$CHOICE" = ".." ]; then
            if [ "$CURRENT_PATH" != "/" ]; then
                CURRENT_PATH=$(dirname "$CURRENT_PATH")
                [ "$CURRENT_PATH" = "." ] && CURRENT_PATH="/"
            fi
            continue
        fi
        
        # Validate numeric choice
        case "$CHOICE" in
            ''|*[!0-9]*) 
                echo "Invalid selection"
                sleep 1
                continue
                ;;
        esac
        
        if [ "$CHOICE" -lt 1 ] || [ "$CHOICE" -gt "$ITEM_COUNT" ]; then
            echo "Invalid selection"
            sleep 1
            continue
        fi
        
        # Get selected item
        SELECTED=$(echo "$ITEMS" | sed -n "${CHOICE}p")
        
        case "$SELECTED" in
            "📁 "*)
                # Enter folder
                FOLDER=$(echo "$SELECTED" | sed 's/^📁 //' | sed 's/\/$//')
                if [ "$CURRENT_PATH" = "/" ]; then
                    CURRENT_PATH="/$FOLDER"
                else
                    CURRENT_PATH="$CURRENT_PATH/$FOLDER"
                fi
                ;;
            "📄 "*)
                # Run script
                SCRIPT=$(echo "$SELECTED" | sed 's/^📄 //')
                if [ "$CURRENT_PATH" = "/" ]; then
                    SCRIPT_PATH="/$SCRIPT"
                else
                    SCRIPT_PATH="$CURRENT_PATH/$SCRIPT"
                fi
                echo ""
                echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
                echo "Running: ${BASE_URL}${SCRIPT_PATH}"
                echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
                echo ""
                curl -fsSL "${BASE_URL}${SCRIPT_PATH}" | sh
                exit 0
                ;;
            *)
                echo "Invalid selection"
                sleep 1
                ;;
        esac
    done
}

# Main
CATALOG=$(fetch_catalog)
if [ -z "$CATALOG" ]; then
    echo "Failed to fetch catalog from ${BASE_URL}" >&2
    exit 1
fi

if has_cmd fzf; then
    browse_fzf
elif has_cmd whiptail; then
    browse_dialog whiptail
elif has_cmd dialog; then
    browse_dialog dialog
else
    browse_fallback
fi
`, s.Hostname)
	
	w.Write([]byte(script))
}

// HandleScript serves a script by path
func (s *Server) HandleScript(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	
	// Ensure path starts with / and ends with .sh
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	
	q := dbgen.New(s.DB)
	script, err := q.GetScriptByPath(r.Context(), path)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}
	
	// Check if preview mode
	if r.URL.Query().Get("preview") == "1" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		// Return metadata/description for preview
		fmt.Fprintf(w, "# %s\n", script.Name)
		if script.Description != nil && *script.Description != "" {
			fmt.Fprintf(w, "# %s\n", *script.Description)
		}
		if script.Tags != nil && *script.Tags != "" {
			fmt.Fprintf(w, "# Tags: %s\n", *script.Tags)
		}
		fmt.Fprintf(w, "\n# Content:\n")
		// Show first 20 lines
		lines := strings.Split(script.Content, "\n")
		max := 20
		if len(lines) < max {
			max = len(lines)
		}
		for i := 0; i < max; i++ {
			fmt.Fprintf(w, "%s\n", lines[i])
		}
		if len(lines) > max {
			fmt.Fprintf(w, "\n... (%d more lines)\n", len(lines)-max)
		}
		return
	}
	
	// Check if script is locked
	if script.Locked != 0 {
		// Check for valid token
		token := r.URL.Query().Get("token")
		if token == "" {
			token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		
		if token != "" {
			// Validate token
			authToken, err := q.GetAuthToken(r.Context(), token)
			if err == nil && authToken.ScriptID == script.ID && authToken.ExpiresAt.After(time.Now()) {
				// Token valid, serve script
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.Header().Set("Cache-Control", "no-store")
				w.Write([]byte(script.Content))
				return
			}
		}
		
		// Serve password prompt script
		s.servePasswordPrompt(w, path)
		return
	}
	
	// Serve script content
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=60")
	w.Write([]byte(script.Content))
}

// servePasswordPrompt serves a script that prompts for password
func (s *Server) servePasswordPrompt(w http.ResponseWriter, scriptPath string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	
	script := fmt.Sprintf(`#!/bin/sh
# This script is locked and requires authentication
set -e

BASE_URL="https://%s"
SCRIPT_PATH="%s"

echo "This script is locked. Please enter the password."
echo ""

# Try to hide input
if [ -t 0 ]; then
    # Terminal available, try to hide password
    stty_orig=$(stty -g 2>/dev/null || true)
    if [ -n "$stty_orig" ]; then
        stty -echo 2>/dev/null || true
        trap 'stty "$stty_orig" 2>/dev/null' EXIT INT TERM
    fi
    printf "Password: "
    read -r PASSWORD
    if [ -n "$stty_orig" ]; then
        stty "$stty_orig" 2>/dev/null || true
    fi
    echo ""
else
    # No terminal, read normally
    printf "Password: "
    read -r PASSWORD
fi

if [ -z "$PASSWORD" ]; then
    echo "Error: Password required"
    exit 1
fi

# Request token
RESPONSE=$(curl -fsSL -X POST "${BASE_URL}/_auth/unlock" \
    -H "Content-Type: application/json" \
    -d "{\"path\":\"${SCRIPT_PATH}\",\"password\":\"${PASSWORD}\"}" 2>&1) || {
    echo "Authentication failed: ${RESPONSE}"
    exit 1
}

# Extract token from JSON response
TOKEN=$(echo "$RESPONSE" | grep -o '"token":"[^"]*"' | sed 's/"token":"\([^"]*\)"/\1/')

if [ -z "$TOKEN" ]; then
    echo "Authentication failed: Invalid response"
    echo "$RESPONSE"
    exit 1
fi

echo "Authentication successful. Running script..."
echo ""

# Fetch and execute the actual script
curl -fsSL "${BASE_URL}${SCRIPT_PATH}?token=${TOKEN}" | sh
`, s.Hostname, scriptPath)
	
	w.Write([]byte(script))
}

// HandleUnlock handles password verification and token generation
func (s *Server) HandleUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		Path     string `json:"path"`
		Password string `json:"password"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	q := dbgen.New(s.DB)
	script, err := q.GetScriptByPath(r.Context(), req.Path)
	if err != nil {
		http.Error(w, "Script not found", http.StatusNotFound)
		return
	}
	
	if script.Locked == 0 {
		http.Error(w, "Script is not locked", http.StatusBadRequest)
		return
	}
	
	if script.PasswordHash == nil {
		http.Error(w, "Script has no password set", http.StatusInternalServerError)
		return
	}
	
	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(*script.PasswordHash), []byte(req.Password)); err != nil {
		// Log failed attempt
		q.CreateAuditLog(r.Context(), dbgen.CreateAuditLogParams{
			Action:     "UNLOCK_FAILED",
			EntityType: "script",
			EntityID:   &script.ID,
			EntityPath: &req.Path,
			IpAddress:  strPtr(r.RemoteAddr),
			UserAgent:  strPtr(r.Header.Get("User-Agent")),
			CreatedAt:  time.Now(),
		})
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}
	
	// Generate token
	token := uuid.New().String()
	expiresAt := time.Now().Add(5 * time.Minute)
	
	if err := q.CreateAuthToken(r.Context(), dbgen.CreateAuthTokenParams{
		Token:     token,
		ScriptID:  script.ID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		IpAddress: strPtr(r.RemoteAddr),
		UserAgent: strPtr(r.Header.Get("User-Agent")),
	}); err != nil {
		http.Error(w, "Failed to create token", http.StatusInternalServerError)
		return
	}
	
	// Log successful unlock
	q.CreateAuditLog(r.Context(), dbgen.CreateAuditLogParams{
		Action:     "UNLOCK_SUCCESS",
		EntityType: "script",
		EntityID:   &script.ID,
		EntityPath: &req.Path,
		IpAddress:  strPtr(r.RemoteAddr),
		UserAgent:  strPtr(r.Header.Get("User-Agent")),
		CreatedAt:  time.Now(),
	})
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      token,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

// HandleCatalog returns the script catalog as JSON
func (s *Server) HandleCatalog(w http.ResponseWriter, r *http.Request) {
	q := dbgen.New(s.DB)
	scripts, err := q.ListScripts(r.Context())
	if err != nil {
		http.Error(w, "Failed to list scripts", http.StatusInternalServerError)
		return
	}
	
	type catalogEntry struct {
		Path        string `json:"path"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Tags        string `json:"tags,omitempty"`
		Locked      bool   `json:"locked"`
	}
	
	entries := make([]catalogEntry, len(scripts))
	for i, s := range scripts {
		entries[i] = catalogEntry{
			Path:   s.Path,
			Name:   s.Name,
			Locked: s.Locked != 0,
		}
		if s.Description != nil {
			entries[i].Description = *s.Description
		}
		if s.Tags != nil {
			entries[i].Tags = *s.Tags
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=60")
	json.NewEncoder(w).Encode(entries)
}

func strPtr(s string) *string {
	return &s
}

// Serve starts the HTTP server
func (s *Server) Serve(addr string) error {
	mux := http.NewServeMux()
	
	// Static files
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	
	// Special endpoints
	mux.HandleFunc("GET /help.sh", s.HandleHelp)
	mux.HandleFunc("GET /search.sh", s.HandleSearch)
	mux.HandleFunc("GET /_catalog.json", s.HandleCatalog)
	mux.HandleFunc("POST /_auth/unlock", s.HandleUnlock)
	
	// API endpoints (for UI)
	mux.HandleFunc("GET /api/scripts", s.adminOnly(s.APIListScripts))
	mux.HandleFunc("POST /api/scripts", s.adminOnly(s.APICreateScript))
	mux.HandleFunc("GET /api/scripts/{id}", s.adminOnly(s.APIGetScript))
	mux.HandleFunc("PUT /api/scripts/{id}", s.adminOnly(s.APIUpdateScript))
	mux.HandleFunc("DELETE /api/scripts/{id}", s.adminOnly(s.APIDeleteScript))
	mux.HandleFunc("GET /api/tree", s.adminOnly(s.APIGetTree))
	mux.HandleFunc("GET /api/folders", s.adminOnly(s.APIListFolders))
	mux.HandleFunc("POST /api/folders", s.adminOnly(s.APICreateFolder))
	mux.HandleFunc("DELETE /api/folders/{id}", s.adminOnly(s.APIDeleteFolder))
	mux.HandleFunc("GET /api/search", s.adminOnly(s.APISearch))
	
	// Root and catch-all routes
	mux.HandleFunc("GET /{$}", s.HandleRoot)
	mux.HandleFunc("GET /{path...}", s.routeHandler)
	
	slog.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, s.withLogging(mux))
}

func (s *Server) routeHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	
	// Handle .sh script requests
	if strings.HasSuffix(path, ".sh") {
		s.HandleScript(w, r)
		return
	}
	
	// For browser requests to non-root paths, serve the SPA
	if !isCLI(r) {
		s.serveHTML(w, r)
		return
	}
	
	// CLI request to unknown path
	http.NotFound(w, r)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

func (s *Server) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Admin-Token")
		if token == "" {
			token = r.Header.Get("Authorization")
			token = strings.TrimPrefix(token, "Bearer ")
		}
		
		if s.AdminToken != "" && token != s.AdminToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		next(w, r)
	}
}

func extractName(path string) string {
	return filepath.Base(path)
}

func validatePath(path string) error {
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path must start with /")
	}
	if !strings.HasSuffix(path, ".sh") {
		return fmt.Errorf("path must end with .sh")
	}
	// Check for invalid characters
	validPath := regexp.MustCompile(`^[a-zA-Z0-9_/.-]+$`)
	if !validPath.MatchString(path) {
		return fmt.Errorf("path contains invalid characters")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
