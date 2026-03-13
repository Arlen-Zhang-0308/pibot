// PiBot WebUI - Main Application

interface Message {
    role: 'user' | 'assistant' | 'system';
    content: string;
}

interface WSMessage {
    type: string;
    id?: string;
    payload: any;
}

interface ExecutionResult {
    command: string;
    output: string;
    error?: string;
    exit_code: number;
    duration: string;
    level: string;
    pending: boolean;
    pending_id?: string;
}

interface FileInfo {
    name: string;
    path: string;
    is_dir: boolean;
    size: number;
    mod_time: string;
    mode: string;
}

class PiBot {
    private ws: WebSocket | null = null;
    private messages: Message[] = [];
    private currentProvider: string = 'openai';
    private reconnectAttempts: number = 0;
    private maxReconnectAttempts: number = 5;
    private messageIdCounter: number = 0;
    private pendingCommands: Map<string, ExecutionResult> = new Map();
    private currentPath: string = '';
    private baseDirectory: string = '';

    constructor() {
        this.init();
    }

    private init(): void {
        this.setupWebSocket();
        this.setupEventListeners();
        this.loadConfig();
        this.loadFiles();
    }

    private setupWebSocket(): void {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/api/ws`;
        
        this.ws = new WebSocket(wsUrl);
        
        this.ws.onopen = () => {
            this.updateConnectionStatus(true);
            this.reconnectAttempts = 0;
        };
        
        this.ws.onclose = () => {
            this.updateConnectionStatus(false);
            this.attemptReconnect();
        };
        
        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
        
        this.ws.onmessage = (event) => {
            this.handleWSMessage(JSON.parse(event.data));
        };
    }

    private attemptReconnect(): void {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            this.reconnectAttempts++;
            const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
            setTimeout(() => this.setupWebSocket(), delay);
        }
    }

    private updateConnectionStatus(connected: boolean): void {
        const statusEl = document.getElementById('connection-status');
        const dotEl = document.querySelector('.status-dot');
        
        if (statusEl && dotEl) {
            statusEl.textContent = connected ? 'Connected' : 'Disconnected';
            dotEl.classList.toggle('connected', connected);
        }
    }

    private handleWSMessage(msg: WSMessage): void {
        switch (msg.type) {
            case 'stream':
                this.handleStreamChunk(msg.id!, msg.payload.content);
                break;
            case 'stream_end':
                this.handleStreamEnd(msg.id!);
                break;
            case 'exec_result':
                this.handleExecResult(msg.payload);
                break;
            case 'pending':
                this.handlePendingCommand(msg.payload);
                break;
            case 'error':
                this.showError(msg.payload.error);
                break;
        }
    }

    private handleStreamChunk(id: string, content: string): void {
        const messageEl = document.querySelector(`[data-message-id="${id}"] .message-text`);
        if (messageEl) {
            messageEl.textContent += content;
        }
    }

    private handleStreamEnd(id: string): void {
        const loadingEl = document.querySelector(`[data-message-id="${id}"] .loading-dots`);
        if (loadingEl) {
            loadingEl.remove();
        }
    }

    private handleExecResult(result: ExecutionResult): void {
        this.addTerminalOutput(result);
    }

    private handlePendingCommand(result: ExecutionResult): void {
        this.pendingCommands.set(result.pending_id!, result);
        this.showPendingModal(result);
    }

    private setupEventListeners(): void {
        // Navigation
        document.querySelectorAll('.nav-item[data-view]').forEach(item => {
            item.addEventListener('click', (e) => {
                e.preventDefault();
                const view = (e.currentTarget as HTMLElement).dataset.view;
                if (view) this.switchView(view);
            });
        });

        // Chat form
        const chatForm = document.getElementById('chat-form');
        chatForm?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.sendChatMessage();
        });

        // Auto-resize textarea
        const chatInput = document.getElementById('chat-input') as HTMLTextAreaElement;
        chatInput?.addEventListener('input', () => {
            chatInput.style.height = 'auto';
            chatInput.style.height = Math.min(chatInput.scrollHeight, 150) + 'px';
        });

        // Enter to send (Shift+Enter for newline)
        chatInput?.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.sendChatMessage();
            }
        });

        // Quick actions
        document.querySelectorAll('.quick-action').forEach(btn => {
            btn.addEventListener('click', () => {
                const prompt = (btn as HTMLElement).dataset.prompt;
                if (prompt && chatInput) {
                    chatInput.value = prompt;
                    this.sendChatMessage();
                }
            });
        });

        // Provider select
        const providerSelect = document.getElementById('provider') as HTMLSelectElement;
        providerSelect?.addEventListener('change', () => {
            this.currentProvider = providerSelect.value;
        });

        // Terminal form
        const terminalForm = document.getElementById('terminal-form');
        terminalForm?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.executeCommand();
        });

        // Clear terminal
        document.getElementById('clear-terminal')?.addEventListener('click', () => {
            const output = document.getElementById('terminal-output');
            if (output) output.innerHTML = '';
        });

        // File refresh
        document.getElementById('refresh-files')?.addEventListener('click', () => {
            this.loadFiles();
        });

        // Modal handlers
        document.getElementById('confirm-command')?.addEventListener('click', () => {
            this.confirmPendingCommand();
        });

        document.getElementById('cancel-command')?.addEventListener('click', () => {
            this.cancelPendingCommand();
        });
    }

    private switchView(viewName: string): void {
        // Update nav
        document.querySelectorAll('.nav-item').forEach(item => {
            item.classList.toggle('active', item.getAttribute('data-view') === viewName);
        });

        // Update views
        document.querySelectorAll('.view').forEach(view => {
            view.classList.toggle('active', view.id === `${viewName}-view`);
        });
    }

    private async loadConfig(): Promise<void> {
        try {
            const response = await fetch('/api/config');
            const config = await response.json();
            
            this.currentProvider = config.default_provider;
            this.baseDirectory = config.base_directory;
            
            const providerSelect = document.getElementById('provider') as HTMLSelectElement;
            if (providerSelect) {
                providerSelect.value = this.currentProvider;
            }
        } catch (error) {
            console.error('Failed to load config:', error);
        }
    }

    private sendChatMessage(): void {
        const input = document.getElementById('chat-input') as HTMLTextAreaElement;
        const content = input.value.trim();
        
        if (!content || !this.ws || this.ws.readyState !== WebSocket.OPEN) return;

        // Hide welcome message
        const welcome = document.querySelector('.welcome-message');
        if (welcome) welcome.remove();

        // Add user message
        this.addMessage('user', content);
        this.messages.push({ role: 'user', content });

        // Clear input
        input.value = '';
        input.style.height = 'auto';

        // Generate message ID
        const messageId = `msg-${++this.messageIdCounter}`;

        // Add assistant message placeholder
        this.addMessage('assistant', '', messageId);

        // Send via WebSocket
        const wsMessage: WSMessage = {
            type: 'chat',
            id: messageId,
            payload: {
                messages: this.messages,
                provider: this.currentProvider
            }
        };

        this.ws.send(JSON.stringify(wsMessage));
    }

    private addMessage(role: 'user' | 'assistant', content: string, id?: string): void {
        const container = document.getElementById('chat-messages');
        if (!container) return;

        const messageEl = document.createElement('div');
        messageEl.className = `message ${role}`;
        if (id) messageEl.dataset.messageId = id;

        const avatar = role === 'assistant' 
            ? `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"/>
                <circle cx="12" cy="12" r="4"/>
               </svg>`
            : `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
                <circle cx="12" cy="7" r="4"/>
               </svg>`;

        messageEl.innerHTML = `
            <div class="message-avatar">${avatar}</div>
            <div class="message-content">
                <p class="message-text">${this.escapeHtml(content)}</p>
                ${role === 'assistant' && !content ? '<span class="loading-dots">Thinking</span>' : ''}
            </div>
        `;

        container.appendChild(messageEl);
        container.scrollTop = container.scrollHeight;
    }

    private executeCommand(): void {
        const input = document.getElementById('terminal-input') as HTMLInputElement;
        const command = input.value.trim();
        
        if (!command) return;

        // Add command to output
        this.addTerminalLine(command, 'command');
        input.value = '';

        // Execute via API or WebSocket
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            const wsMessage: WSMessage = {
                type: 'exec',
                id: `exec-${++this.messageIdCounter}`,
                payload: { command }
            };
            this.ws.send(JSON.stringify(wsMessage));
        } else {
            // Fallback to HTTP
            this.executeCommandHttp(command);
        }
    }

    private async executeCommandHttp(command: string): Promise<void> {
        try {
            const response = await fetch('/api/exec', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ command })
            });
            
            const result: ExecutionResult = await response.json();
            
            if (result.pending) {
                this.handlePendingCommand(result);
            } else {
                this.addTerminalOutput(result);
            }
        } catch (error) {
            this.addTerminalLine(`Error: ${error}`, 'error');
        }
    }

    private addTerminalOutput(result: ExecutionResult): void {
        if (result.output) {
            this.addTerminalLine(result.output, 'output');
        }
        if (result.error) {
            this.addTerminalLine(result.error, 'error');
        }
        if (result.exit_code !== 0) {
            this.addTerminalLine(`Exit code: ${result.exit_code}`, 'error');
        }
    }

    private addTerminalLine(content: string, type: 'command' | 'output' | 'error' | 'success'): void {
        const output = document.getElementById('terminal-output');
        if (!output) return;

        const line = document.createElement('div');
        line.className = `terminal-line ${type}`;
        line.textContent = content;
        output.appendChild(line);
        output.scrollTop = output.scrollHeight;
    }

    private showPendingModal(result: ExecutionResult): void {
        const modal = document.getElementById('pending-modal');
        const commandEl = document.getElementById('pending-command');
        
        if (modal && commandEl) {
            commandEl.textContent = result.command;
            modal.dataset.pendingId = result.pending_id;
            modal.classList.remove('hidden');
        }
    }

    private async confirmPendingCommand(): Promise<void> {
        const modal = document.getElementById('pending-modal');
        const pendingId = modal?.dataset.pendingId;
        
        if (!pendingId) return;

        try {
            const response = await fetch(`/api/exec/confirm/${pendingId}`, {
                method: 'POST'
            });
            const result: ExecutionResult = await response.json();
            this.addTerminalOutput(result);
        } catch (error) {
            this.showError(`Failed to execute: ${error}`);
        }

        this.closePendingModal();
    }

    private async cancelPendingCommand(): Promise<void> {
        const modal = document.getElementById('pending-modal');
        const pendingId = modal?.dataset.pendingId;
        
        if (!pendingId) return;

        try {
            await fetch(`/api/exec/cancel/${pendingId}`, { method: 'POST' });
            this.addTerminalLine('Command cancelled', 'success');
        } catch (error) {
            this.showError(`Failed to cancel: ${error}`);
        }

        this.closePendingModal();
    }

    private closePendingModal(): void {
        const modal = document.getElementById('pending-modal');
        if (modal) {
            modal.classList.add('hidden');
            delete modal.dataset.pendingId;
        }
    }

    private async loadFiles(path?: string): Promise<void> {
        try {
            const url = path ? `/api/files?path=${encodeURIComponent(path)}` : '/api/files';
            const response = await fetch(url);
            const data = await response.json();
            
            this.currentPath = path || '';
            this.baseDirectory = data.base_directory;
            this.renderFiles(data.files);
            this.updateBreadcrumb();
        } catch (error) {
            console.error('Failed to load files:', error);
        }
    }

    private renderFiles(files: FileInfo[]): void {
        const container = document.getElementById('files-list');
        if (!container) return;

        if (files.length === 0) {
            container.innerHTML = '<div class="file-item"><span class="file-name" style="color: var(--text-muted);">Empty directory</span></div>';
            return;
        }

        // Sort: directories first, then files
        files.sort((a, b) => {
            if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
            return a.name.localeCompare(b.name);
        });

        container.innerHTML = files.map(file => `
            <div class="file-item ${file.is_dir ? 'directory' : 'file'}" data-path="${this.escapeHtml(file.path)}" data-isdir="${file.is_dir}">
                ${file.is_dir 
                    ? `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
                       </svg>`
                    : `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                        <polyline points="14 2 14 8 20 8"/>
                       </svg>`
                }
                <div class="file-info">
                    <div class="file-name">${this.escapeHtml(file.name)}</div>
                    <div class="file-meta">${file.is_dir ? 'Directory' : this.formatSize(file.size)} • ${file.mod_time}</div>
                </div>
            </div>
        `).join('');

        // Add click handlers
        container.querySelectorAll('.file-item').forEach(item => {
            item.addEventListener('click', () => {
                const path = (item as HTMLElement).dataset.path;
                const isDir = (item as HTMLElement).dataset.isdir === 'true';
                
                if (isDir && path) {
                    this.loadFiles(path);
                } else if (path) {
                    this.openFile(path);
                }
            });
        });
    }

    private updateBreadcrumb(): void {
        const breadcrumb = document.getElementById('breadcrumb');
        if (!breadcrumb) return;

        const parts = this.currentPath ? this.currentPath.split('/').filter(Boolean) : [];
        let html = `<span class="breadcrumb-item" data-path="">~/pibot-workspace</span>`;
        
        let currentPath = this.baseDirectory;
        for (const part of parts) {
            currentPath += '/' + part;
            html += ` / <span class="breadcrumb-item" data-path="${this.escapeHtml(currentPath)}">${this.escapeHtml(part)}</span>`;
        }

        breadcrumb.innerHTML = html;

        // Add click handlers
        breadcrumb.querySelectorAll('.breadcrumb-item').forEach(item => {
            item.addEventListener('click', () => {
                const path = (item as HTMLElement).dataset.path;
                this.loadFiles(path || undefined);
            });
        });
    }

    private async openFile(path: string): Promise<void> {
        // For now, just show file content in an alert. In a full implementation,
        // this would open a modal with an editor.
        try {
            const relativePath = path.replace(this.baseDirectory + '/', '');
            const response = await fetch(`/api/files/${encodeURIComponent(relativePath)}`);
            const data = await response.json();
            
            // Switch to chat and show content
            this.switchView('chat');
            this.addMessage('assistant', `**File: ${path}**\n\n\`\`\`\n${data.content}\n\`\`\``);
        } catch (error) {
            this.showError(`Failed to read file: ${error}`);
        }
    }

    private formatSize(bytes: number): string {
        const units = ['B', 'KB', 'MB', 'GB'];
        let size = bytes;
        let unitIndex = 0;
        
        while (size >= 1024 && unitIndex < units.length - 1) {
            size /= 1024;
            unitIndex++;
        }
        
        return `${size.toFixed(unitIndex > 0 ? 1 : 0)} ${units[unitIndex]}`;
    }

    private showError(message: string): void {
        console.error(message);
        // In a full implementation, this would show a toast notification
        alert(message);
    }

    private escapeHtml(text: string): string {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize the application
document.addEventListener('DOMContentLoaded', () => {
    new PiBot();
});
