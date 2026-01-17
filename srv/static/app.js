// SH Server Admin UI
(function() {
    'use strict';

    let adminToken = localStorage.getItem('adminToken') || '';
    let currentScript = null;
    let scripts = [];
    let folders = [];
    let draggedScript = null;
    let contextMenuFolder = null;

    const $ = (sel) => document.querySelector(sel);
    const $$ = (sel) => document.querySelectorAll(sel);

    // API helper
    async function api(method, path, body = null) {
        const opts = {
            method,
            headers: {
                'Content-Type': 'application/json',
                'X-Admin-Token': adminToken
            }
        };
        if (body) {
            opts.body = JSON.stringify(body);
        }
        const res = await fetch(path, opts);
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || res.statusText);
        }
        if (res.status === 204) return null;
        return res.json();
    }

    // Initialize
    async function init() {
        // Check if we have a valid token
        if (adminToken) {
            try {
                await api('GET', '/api/scripts');
                $('#auth-modal').classList.remove('active');
                await loadData();
            } catch (e) {
                adminToken = '';
                localStorage.removeItem('adminToken');
            }
        }

        // Auth modal
        $('#btn-auth').addEventListener('click', async () => {
            adminToken = $('#admin-token').value;
            try {
                await api('GET', '/api/scripts');
                localStorage.setItem('adminToken', adminToken);
                $('#auth-modal').classList.remove('active');
                await loadData();
            } catch (e) {
                alert('Invalid token');
            }
        });

        $('#admin-token').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') $('#btn-auth').click();
        });

        // Logo click - go to home
        $('#logo-link').addEventListener('click', (e) => {
            e.preventDefault();
            currentScript = null;
            showWelcome();
            // Clear active state
            $$('.tree-item').forEach(el => el.classList.remove('active'));
        });

        // New script button
        $('#btn-new-script').addEventListener('click', () => {
            currentScript = null;
            showEditor({
                path: '/new-script.sh',
                content: '#!/bin/sh\n# New script\n',
                description: '',
                tags: '',
                requires: '',
                locked: false,
                danger_level: 0
            });
        });

        // New folder button
        $('#btn-new-folder').addEventListener('click', () => {
            $('#folder-modal').classList.add('active');
            $('#new-folder-path').value = '/';
            $('#new-folder-path').focus();
        });

        $('#btn-create-folder').addEventListener('click', async () => {
            const path = $('#new-folder-path').value;
            if (!path || !path.startsWith('/')) {
                alert('Path must start with /');
                return;
            }
            try {
                await api('POST', '/api/folders', { path });
                $('#folder-modal').classList.remove('active');
                await loadData();
            } catch (e) {
                alert('Failed to create folder: ' + e.message);
            }
        });

        $('#btn-cancel-folder').addEventListener('click', () => {
            $('#folder-modal').classList.remove('active');
        });

        // Save button
        $('#btn-save').addEventListener('click', saveScript);

        // Delete button
        $('#btn-delete').addEventListener('click', async () => {
            if (!currentScript || !currentScript.id) return;
            if (!confirm('Delete this script?')) return;
            try {
                await api('DELETE', `/api/scripts/${currentScript.id}`);
                currentScript = null;
                showWelcome();
                await loadData();
            } catch (e) {
                alert('Failed to delete: ' + e.message);
            }
        });

        // Search
        let searchTimeout;
        $('#search-input').addEventListener('input', (e) => {
            clearTimeout(searchTimeout);
            searchTimeout = setTimeout(() => filterTree(e.target.value), 200);
        });

        // Update curl command on path change
        $('#script-path').addEventListener('input', updateCurlCommand);

        // Copy curl command button
        $('#btn-copy-curl').addEventListener('click', () => {
            const curlCommand = $('#curl-command').textContent;
            if (curlCommand) {
                navigator.clipboard.writeText(curlCommand).then(() => {
                    const btn = $('#btn-copy-curl');
                    btn.textContent = '✅';
                    btn.classList.add('copied');
                    setTimeout(() => {
                        btn.textContent = '📋';
                        btn.classList.remove('copied');
                    }, 1500);
                }).catch(() => {
                    alert('Failed to copy to clipboard');
                });
            }
        });

        // Context menu for folder deletion
        $('#ctx-delete-folder').addEventListener('click', async () => {
            if (!contextMenuFolder) return;
            hideContextMenu();
            
            if (!confirm(`Delete folder "${contextMenuFolder.path}"? This will NOT delete scripts inside.`)) {
                return;
            }
            
            try {
                await api('DELETE', `/api/folders/${contextMenuFolder.id}`);
                await loadData();
            } catch (e) {
                alert('Failed to delete folder: ' + e.message);
            }
            contextMenuFolder = null;
        });

        // Hide context menu on click elsewhere
        document.addEventListener('click', (e) => {
            if (!e.target.closest('.context-menu')) {
                hideContextMenu();
            }
        });
    }

    async function loadData() {
        try {
            scripts = await api('GET', '/api/scripts');
            folders = await api('GET', '/api/folders');
            renderTree();
        } catch (e) {
            console.error('Failed to load data:', e);
        }
    }

    function renderTree() {
        const container = $('#tree-container');
        
        // Build tree structure
        const tree = { children: {}, scripts: [] };
        
        // Add folders
        folders.forEach(f => {
            const parts = f.path.split('/').filter(Boolean);
            let node = tree;
            parts.forEach((part, i) => {
                if (!node.children[part]) {
                    node.children[part] = { 
                        id: i === parts.length - 1 ? f.id : null,
                        name: part, 
                        path: '/' + parts.slice(0, i + 1).join('/'),
                        children: {}, 
                        scripts: [] 
                    };
                }
                node = node.children[part];
            });
        });
        
        // Add scripts
        scripts.forEach(s => {
            const parts = s.path.split('/').filter(Boolean);
            const fileName = parts.pop();
            let node = tree;
            parts.forEach((part, i) => {
                if (!node.children[part]) {
                    node.children[part] = {
                        name: part,
                        path: '/' + parts.slice(0, i + 1).join('/'),
                        children: {},
                        scripts: []
                    };
                }
                node = node.children[part];
            });
            node.scripts.push(s);
        });
        
        container.innerHTML = renderTreeNode(tree, true);
        
        // Add click handlers
        container.querySelectorAll('.tree-item.script').forEach(el => {
            el.addEventListener('click', () => {
                const id = el.dataset.id;
                const script = scripts.find(s => s.id === id);
                if (script) {
                    currentScript = script;
                    showEditor(script);
                    // Update active state
                    container.querySelectorAll('.tree-item').forEach(e => e.classList.remove('active'));
                    el.classList.add('active');
                }
            });
        });

        container.querySelectorAll('.tree-item.folder').forEach(el => {
            el.addEventListener('click', (e) => {
                if (e.button !== 0) return; // Only left click
                const children = el.nextElementSibling;
                if (children && children.classList.contains('tree-children')) {
                    children.style.display = children.style.display === 'none' ? 'block' : 'none';
                    el.querySelector('.icon').textContent = children.style.display === 'none' ? '📁' : '📂';
                }
            });
        });

        // Setup drag and drop
        setupDragAndDrop(container);
    }

    function renderTreeNode(node, isRoot = false) {
        let html = '';
        
        // Render folders
        const folderKeys = Object.keys(node.children).sort();
        folderKeys.forEach(key => {
            const folder = node.children[key];
            const folderId = folder.id ? `data-folder-id="${folder.id}"` : '';
            html += `<div class="tree-item folder" data-path="${folder.path}" ${folderId}>
                <span class="icon">📂</span>
                <span class="name">${folder.name}</span>
            </div>`;
            html += `<div class="tree-children" data-folder-path="${folder.path}">${renderTreeNode(folder)}</div>`;
        });
        
        // Render scripts
        if (node.scripts) {
            node.scripts.sort((a, b) => a.name.localeCompare(b.name)).forEach(s => {
                const lockedClass = s.locked ? ' locked' : '';
                html += `<div class="tree-item script${lockedClass}" data-id="${s.id}" data-path="${s.path}" draggable="true">
                    <span class="icon">📄</span>
                    <span class="name">${s.name}</span>
                </div>`;
            });
        }
        
        return html;
    }

    function hideContextMenu() {
        $('#folder-context-menu').classList.remove('active');
    }

    function showContextMenu(x, y) {
        const menu = $('#folder-context-menu');
        menu.style.left = x + 'px';
        menu.style.top = y + 'px';
        menu.classList.add('active');
    }

    function setupDragAndDrop(container) {
        // Script drag start
        container.querySelectorAll('.tree-item.script').forEach(el => {
            el.addEventListener('dragstart', (e) => {
                draggedScript = scripts.find(s => s.id === el.dataset.id);
                el.classList.add('dragging');
                e.dataTransfer.effectAllowed = 'move';
            });

            el.addEventListener('dragend', () => {
                el.classList.remove('dragging');
                container.querySelectorAll('.drag-over').forEach(e => e.classList.remove('drag-over'));
                draggedScript = null;
            });
        });

        // Folder drop targets
        container.querySelectorAll('.tree-item.folder').forEach(el => {
            el.addEventListener('dragover', (e) => {
                e.preventDefault();
                if (draggedScript) {
                    el.classList.add('drag-over');
                }
            });

            el.addEventListener('dragleave', () => {
                el.classList.remove('drag-over');
            });

            el.addEventListener('drop', async (e) => {
                e.preventDefault();
                el.classList.remove('drag-over');
                
                if (!draggedScript) return;
                
                const targetFolder = el.dataset.path;
                const newPath = targetFolder + '/' + draggedScript.name;
                
                if (newPath === draggedScript.path) return;
                
                try {
                    await api('PUT', `/api/scripts/${draggedScript.id}`, {
                        ...draggedScript,
                        path: newPath
                    });
                    await loadData();
                } catch (e) {
                    alert('Failed to move script: ' + e.message);
                }
            });

            // Right-click for folder context menu
            el.addEventListener('contextmenu', (e) => {
                e.preventDefault();
                const folderId = el.dataset.folderId;
                if (folderId) {
                    contextMenuFolder = { id: folderId, path: el.dataset.path };
                    showContextMenu(e.clientX, e.clientY);
                }
            });
        });

        // Root drop target (move to root)
        container.addEventListener('dragover', (e) => {
            if (e.target === container && draggedScript) {
                e.preventDefault();
            }
        });

        container.addEventListener('drop', async (e) => {
            if (e.target !== container || !draggedScript) return;
            e.preventDefault();
            
            const newPath = '/' + draggedScript.name;
            if (newPath === draggedScript.path) return;
            
            try {
                await api('PUT', `/api/scripts/${draggedScript.id}`, {
                    ...draggedScript,
                    path: newPath
                });
                await loadData();
            } catch (e) {
                alert('Failed to move script: ' + e.message);
            }
        });
    }

    function filterTree(query) {
        if (!query) {
            renderTree();
            return;
        }
        
        const q = query.toLowerCase();
        const container = $('#tree-container');
        
        container.querySelectorAll('.tree-item.script').forEach(el => {
            const path = el.dataset.path.toLowerCase();
            const script = scripts.find(s => s.id === el.dataset.id);
            const matches = path.includes(q) || 
                           (script && script.description && script.description.toLowerCase().includes(q)) ||
                           (script && script.tags && script.tags.toLowerCase().includes(q));
            el.style.display = matches ? '' : 'none';
        });
        
        // Show parent folders if any child is visible
        container.querySelectorAll('.tree-children').forEach(el => {
            const hasVisible = el.querySelector('.tree-item.script:not([style*="display: none"])');
            if (el.previousElementSibling) {
                el.previousElementSibling.style.display = hasVisible ? '' : 'none';
            }
            el.style.display = hasVisible ? '' : 'none';
        });
    }

    function showWelcome() {
        $('#welcome-view').classList.add('active');
        $('#editor-view').classList.remove('active');
    }

    function showEditor(script) {
        $('#welcome-view').classList.remove('active');
        $('#editor-view').classList.add('active');
        
        $('#script-path').value = script.path || '';
        $('#script-content').value = script.content || '';
        $('#script-description').value = script.description || '';
        $('#script-tags').value = script.tags || '';
        $('#script-requires').value = script.requires || '';
        $('#script-locked').checked = script.locked || false;
        $('#script-password').value = '';
        $('#script-danger').value = script.danger_level || 0;
        
        updateCurlCommand();
        updateScriptInfo();
    }

    function updateCurlCommand() {
        const path = $('#script-path').value;
        if (path && path.endsWith('.sh')) {
            $('#curl-command').textContent = `curl -fsSL https://sh.huny.dev${path} | sh`;
        } else {
            $('#curl-command').textContent = '';
        }
    }

    function updateScriptInfo() {
        if (currentScript && currentScript.id) {
            const updated = new Date(currentScript.updated_at).toLocaleString();
            $('#script-info').textContent = `Last updated: ${updated}`;
        } else {
            $('#script-info').textContent = 'New script';
        }
    }

    async function saveScript() {
        const data = {
            path: $('#script-path').value,
            content: $('#script-content').value,
            description: $('#script-description').value,
            tags: $('#script-tags').value,
            requires: $('#script-requires').value,
            locked: $('#script-locked').checked,
            password: $('#script-password').value,
            danger_level: parseInt($('#script-danger').value) || 0
        };
        
        if (!data.path || !data.path.startsWith('/') || !data.path.endsWith('.sh')) {
            alert('Path must start with / and end with .sh');
            return;
        }
        
        try {
            let result;
            if (currentScript && currentScript.id) {
                result = await api('PUT', `/api/scripts/${currentScript.id}`, data);
            } else {
                result = await api('POST', '/api/scripts', data);
            }
            currentScript = result;
            updateScriptInfo();
            await loadData();
            alert('Saved!');
        } catch (e) {
            alert('Failed to save: ' + e.message);
        }
    }

    // Start
    document.addEventListener('DOMContentLoaded', init);
})();
