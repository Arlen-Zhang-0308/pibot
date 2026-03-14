const { createApp, ref, reactive, computed, nextTick, onMounted, watch } = Vue;

createApp({
    setup() {
        // ── Navigation ──────────────────────────────────────────────────
        const currentView = ref('chat');

        // ── WebSocket ────────────────────────────────────────────────────
        const wsConnected = ref(false);
        let ws = null;
        let reconnectAttempts = 0;
        const maxReconnectAttempts = 5;
        let messageIdCounter = 0;

        function setupWebSocket() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/api/ws`;
            ws = new WebSocket(wsUrl);

            ws.onopen = () => {
                wsConnected.value = true;
                reconnectAttempts = 0;
            };

            ws.onclose = () => {
                wsConnected.value = false;
                attemptReconnect();
            };

            ws.onerror = (err) => {
                console.error('WebSocket error:', err);
            };

            ws.onmessage = (event) => {
                handleWSMessage(JSON.parse(event.data));
            };
        }

        function attemptReconnect() {
            if (reconnectAttempts < maxReconnectAttempts) {
                reconnectAttempts++;
                const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000);
                setTimeout(setupWebSocket, delay);
            }
        }

        function handleWSMessage(msg) {
            switch (msg.type) {
                case 'stream':
                    handleStreamChunk(msg.id, msg.payload.content);
                    break;
                case 'stream_end':
                    handleStreamEnd(msg.id);
                    break;
                case 'exec_result':
                    handleExecResult(msg.payload);
                    break;
                case 'pending':
                    handlePendingCommand(msg.payload);
                    break;
                case 'error':
                    console.error('Server error:', msg.payload.error);
                    break;
            }
        }

        // ── Chat ─────────────────────────────────────────────────────────
        const messages = ref([]);
        const chatInput = ref('');
        const chatFocused = ref(false);
        const chatMessagesEl = ref(null);
        const chatInputEl = ref(null);
        const currentProvider = ref('qwen');

        function scrollChatToBottom() {
            nextTick(() => {
                if (chatMessagesEl.value) {
                    chatMessagesEl.value.scrollTop = chatMessagesEl.value.scrollHeight;
                }
            });
        }

        function autoResizeTextarea() {
            const el = chatInputEl.value;
            if (el) {
                el.style.height = 'auto';
                el.style.height = Math.min(el.scrollHeight, 150) + 'px';
            }
        }

        function quickAction(prompt) {
            chatInput.value = prompt;
            sendChatMessage();
        }

        function sendChatMessage() {
            const content = chatInput.value.trim();
            if (!content || !ws || ws.readyState !== WebSocket.OPEN) return;

            const msgId = `msg-${++messageIdCounter}`;

            messages.value.push({ id: `user-${msgId}`, role: 'user', content, loading: false });
            messages.value.push({ id: msgId, role: 'assistant', content: '', loading: true });

            chatInput.value = '';
            nextTick(() => {
                if (chatInputEl.value) chatInputEl.value.style.height = 'auto';
                scrollChatToBottom();
            });

            const history = messages.value
                .filter(m => !m.loading)
                .map(m => ({ role: m.role, content: m.content }));

            ws.send(JSON.stringify({
                type: 'chat',
                id: msgId,
                payload: { messages: history, provider: currentProvider.value }
            }));
        }

        function handleStreamChunk(id, content) {
            const msg = messages.value.find(m => m.id === id);
            if (msg) {
                msg.content += content;
                scrollChatToBottom();
            }
        }

        function handleStreamEnd(id) {
            const msg = messages.value.find(m => m.id === id);
            if (msg) {
                msg.loading = false;
            }
        }

        // Simple markdown rendering: code blocks, inline code, bold, italic, line breaks
        function renderMarkdown(text) {
            if (!text) return '';
            let html = text
                // code blocks
                .replace(/```(\w*)\n?([\s\S]*?)```/g, (_, lang, code) =>
                    `<pre><code>${escapeHtml(code.trim())}</code></pre>`)
                // inline code
                .replace(/`([^`]+)`/g, (_, c) => `<code>${escapeHtml(c)}</code>`)
                // bold
                .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
                // italic
                .replace(/\*(.+?)\*/g, '<em>$1</em>')
                // newlines
                .replace(/\n/g, '<br>');
            return html;
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        // ── Terminal ─────────────────────────────────────────────────────
        const terminalLines = ref([]);
        const terminalInput = ref('');
        const terminalOutputEl = ref(null);
        const terminalInputEl = ref(null);

        function scrollTerminalToBottom() {
            nextTick(() => {
                if (terminalOutputEl.value) {
                    terminalOutputEl.value.scrollTop = terminalOutputEl.value.scrollHeight;
                }
            });
        }

        function clearTerminal() {
            terminalLines.value = [];
        }

        function executeCommand() {
            const command = terminalInput.value.trim();
            if (!command) return;

            terminalLines.value.push({ type: 'command', content: command });
            terminalInput.value = '';
            scrollTerminalToBottom();

            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({
                    type: 'exec',
                    id: `exec-${++messageIdCounter}`,
                    payload: { command }
                }));
            } else {
                executeCommandHttp(command);
            }
        }

        async function executeCommandHttp(command) {
            try {
                const resp = await fetch('/api/exec', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ command })
                });
                const result = await resp.json();
                if (result.pending) {
                    handlePendingCommand(result);
                } else {
                    addTerminalOutput(result);
                }
            } catch (err) {
                terminalLines.value.push({ type: 'error', content: `Error: ${err}` });
                scrollTerminalToBottom();
            }
        }

        function handleExecResult(result) {
            addTerminalOutput(result);
        }

        function addTerminalOutput(result) {
            if (result.output) terminalLines.value.push({ type: 'output', content: result.output });
            if (result.error) terminalLines.value.push({ type: 'error', content: result.error });
            if (result.exit_code !== 0) terminalLines.value.push({ type: 'error', content: `Exit code: ${result.exit_code}` });
            scrollTerminalToBottom();
        }

        // ── Files ─────────────────────────────────────────────────────────
        const fileList = ref([]);
        const currentFilePath = ref('');
        const baseDirectory = ref('');
        const breadcrumbs = computed(() => {
            if (!currentFilePath.value) return [];
            const parts = currentFilePath.value.split('/').filter(Boolean);
            let base = baseDirectory.value;
            return parts.map(part => {
                base = base + '/' + part;
                return { name: part, path: base };
            });
        });

        async function loadFiles(path) {
            try {
                const url = path ? `/api/files?path=${encodeURIComponent(path)}` : '/api/files';
                const resp = await fetch(url);
                const data = await resp.json();
                currentFilePath.value = path || '';
                baseDirectory.value = data.base_directory || '';
                const sorted = (data.files || []).slice().sort((a, b) => {
                    if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
                    return a.name.localeCompare(b.name);
                });
                fileList.value = sorted;
            } catch (err) {
                console.error('Failed to load files:', err);
            }
        }

        function onFileClick(file) {
            if (file.is_dir) {
                loadFiles(file.path);
            } else {
                openFile(file.path);
            }
        }

        async function openFile(path) {
            try {
                const relativePath = path.replace(baseDirectory.value + '/', '');
                const resp = await fetch(`/api/files/${encodeURIComponent(relativePath)}`);
                const data = await resp.json();
                currentView.value = 'chat';
                messages.value.push({
                    id: `file-${++messageIdCounter}`,
                    role: 'assistant',
                    content: `**File: ${path}**\n\n\`\`\`\n${data.content}\n\`\`\``,
                    loading: false
                });
                scrollChatToBottom();
            } catch (err) {
                console.error('Failed to read file:', err);
            }
        }

        function formatSize(bytes) {
            const units = ['B', 'KB', 'MB', 'GB'];
            let size = bytes;
            let i = 0;
            while (size >= 1024 && i < units.length - 1) { size /= 1024; i++; }
            return `${size.toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
        }

        // ── Pending Command Modal ─────────────────────────────────────────
        const pendingModal = reactive({ visible: false, command: '', pendingId: '' });

        function handlePendingCommand(result) {
            pendingModal.command = result.command;
            pendingModal.pendingId = result.pending_id;
            pendingModal.visible = true;
        }

        async function confirmPendingCommand() {
            try {
                const resp = await fetch(`/api/exec/confirm/${pendingModal.pendingId}`, { method: 'POST' });
                const result = await resp.json();
                addTerminalOutput(result);
            } catch (err) {
                console.error('Failed to confirm command:', err);
            }
            pendingModal.visible = false;
        }

        async function cancelPendingCommand() {
            try {
                await fetch(`/api/exec/cancel/${pendingModal.pendingId}`, { method: 'POST' });
                terminalLines.value.push({ type: 'success', content: 'Command cancelled' });
                scrollTerminalToBottom();
            } catch (err) {
                console.error('Failed to cancel command:', err);
            }
            pendingModal.visible = false;
        }

        // ── Settings ──────────────────────────────────────────────────────
        const settingsForm = reactive({
            default_provider: 'openai',
            openai_key: '',
            openai_model: 'gpt-4o',
            anthropic_key: '',
            anthropic_model: 'claude-3-5-sonnet-20241022',
            google_key: '',
            google_model: 'gemini-1.5-pro',
            ollama_base_url: '',
            ollama_model: '',
            qwen_key: '',
            qwen_base_url: '',
            qwen_model: 'qwen-plus',
        });
        const saveStatusMsg = ref('');
        const saveStatusError = ref(false);

        async function loadConfig() {
            try {
                const resp = await fetch('/api/config');
                const cfg = await resp.json();
                if (cfg.default_provider) {
                    settingsForm.default_provider = cfg.default_provider;
                    currentProvider.value = cfg.default_provider;
                }
                if (cfg.openai_model) settingsForm.openai_model = cfg.openai_model;
                if (cfg.anthropic_model) settingsForm.anthropic_model = cfg.anthropic_model;
                if (cfg.google_model) settingsForm.google_model = cfg.google_model;
                if (cfg.ollama_base_url) settingsForm.ollama_base_url = cfg.ollama_base_url;
                if (cfg.ollama_model) settingsForm.ollama_model = cfg.ollama_model;
                if (cfg.qwen_base_url) settingsForm.qwen_base_url = cfg.qwen_base_url;
                if (cfg.qwen_model) settingsForm.qwen_model = cfg.qwen_model;
                if (cfg.base_directory) baseDirectory.value = cfg.base_directory;
            } catch (err) {
                console.error('Failed to load config:', err);
            }
        }

        async function saveSettings() {
            const payload = {};
            if (settingsForm.default_provider) payload.default_provider = settingsForm.default_provider;
            if (settingsForm.openai_key) payload.openai_key = settingsForm.openai_key;
            if (settingsForm.openai_model) payload.openai_model = settingsForm.openai_model;
            if (settingsForm.anthropic_key) payload.anthropic_key = settingsForm.anthropic_key;
            if (settingsForm.anthropic_model) payload.anthropic_model = settingsForm.anthropic_model;
            if (settingsForm.google_key) payload.google_key = settingsForm.google_key;
            if (settingsForm.google_model) payload.google_model = settingsForm.google_model;
            if (settingsForm.ollama_base_url) payload.ollama_base_url = settingsForm.ollama_base_url;
            if (settingsForm.ollama_model) payload.ollama_model = settingsForm.ollama_model;
            if (settingsForm.qwen_key) payload.qwen_key = settingsForm.qwen_key;
            if (settingsForm.qwen_base_url) payload.qwen_base_url = settingsForm.qwen_base_url;
            if (settingsForm.qwen_model) payload.qwen_model = settingsForm.qwen_model;

            try {
                const resp = await fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });
                if (resp.ok) {
                    saveStatusMsg.value = 'Settings saved successfully!';
                    saveStatusError.value = false;
                    settingsForm.openai_key = '';
                    settingsForm.anthropic_key = '';
                    settingsForm.google_key = '';
                    settingsForm.qwen_key = '';
                    currentProvider.value = settingsForm.default_provider;
                } else {
                    const err = await resp.json();
                    saveStatusMsg.value = `Failed to save: ${err.error}`;
                    saveStatusError.value = true;
                }
            } catch (err) {
                saveStatusMsg.value = `Error: ${err}`;
                saveStatusError.value = true;
            }
            setTimeout(() => { saveStatusMsg.value = ''; }, 5000);
        }

        // ── Scheduled Tasks ───────────────────────────────────────────────
        const tasks = ref([]);

        const taskForm = reactive({
            name: '',
            action_type: 'shell',
            command: '',
            schedule: '',
            enabled: true,
        });

        const taskModal = reactive({
            visible: false,
            isEdit: false,
            editId: null,
            error: '',
        });

        const historyDrawer = reactive({
            visible: false,
            taskName: '',
            taskId: null,
            records: [],
        });

        async function loadTasks() {
            try {
                const resp = await fetch('/api/tasks');
                tasks.value = await resp.json() || [];
            } catch (err) {
                console.error('Failed to load tasks:', err);
            }
        }

        function switchToTasks() {
            currentView.value = 'tasks';
            loadTasks();
        }

        function openNewTaskModal() {
            taskForm.name = '';
            taskForm.action_type = 'shell';
            taskForm.command = '';
            taskForm.schedule = '';
            taskForm.enabled = true;
            taskModal.isEdit = false;
            taskModal.editId = null;
            taskModal.error = '';
            taskModal.visible = true;
        }

        function openEditTaskModal(task) {
            taskForm.name = task.name;
            taskForm.action_type = task.action_type;
            taskForm.command = task.command;
            taskForm.schedule = task.schedule;
            taskForm.enabled = task.enabled;
            taskModal.isEdit = true;
            taskModal.editId = task.id;
            taskModal.error = '';
            taskModal.visible = true;
        }

        function closeTaskModal() {
            taskModal.visible = false;
        }

        async function saveTask() {
            taskModal.error = '';
            const payload = {
                name: taskForm.name,
                action_type: taskForm.action_type,
                command: taskForm.command,
                schedule: taskForm.schedule,
                enabled: taskForm.enabled,
            };

            try {
                let resp;
                if (taskModal.isEdit) {
                    resp = await fetch(`/api/tasks/${taskModal.editId}`, {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(payload),
                    });
                } else {
                    resp = await fetch('/api/tasks', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(payload),
                    });
                }

                if (resp.ok) {
                    taskModal.visible = false;
                    await loadTasks();
                } else {
                    const err = await resp.json();
                    taskModal.error = err.error || 'Unknown error';
                }
            } catch (err) {
                taskModal.error = String(err);
            }
        }

        async function deleteTask(task) {
            if (!confirm(`Delete task "${task.name}"?`)) return;
            try {
                await fetch(`/api/tasks/${task.id}`, { method: 'DELETE' });
                await loadTasks();
            } catch (err) {
                console.error('Failed to delete task:', err);
            }
        }

        async function toggleTask(task) {
            try {
                await fetch(`/api/tasks/${task.id}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        name: task.name,
                        action_type: task.action_type,
                        command: task.command,
                        schedule: task.schedule,
                        enabled: !task.enabled,
                    }),
                });
                await loadTasks();
            } catch (err) {
                console.error('Failed to toggle task:', err);
            }
        }

        async function runTaskNow(task) {
            try {
                await fetch(`/api/tasks/${task.id}/run`, { method: 'POST' });
                // Refresh after a short delay to pick up the new run record
                setTimeout(async () => {
                    await loadTasks();
                    if (historyDrawer.visible && historyDrawer.taskId === task.id) {
                        await loadTaskHistory(task.id, task.name);
                    }
                }, 2000);
            } catch (err) {
                console.error('Failed to run task:', err);
            }
        }

        async function loadTaskHistory(taskId, taskName) {
            try {
                const resp = await fetch(`/api/tasks/${taskId}/history`);
                const records = await resp.json() || [];
                historyDrawer.records = records.slice().reverse(); // newest first
                historyDrawer.taskId = taskId;
                historyDrawer.taskName = taskName;
            } catch (err) {
                console.error('Failed to load task history:', err);
            }
        }

        async function openTaskHistory(task) {
            historyDrawer.records = [];
            historyDrawer.taskName = task.name;
            historyDrawer.taskId = task.id;
            historyDrawer.visible = true;
            await loadTaskHistory(task.id, task.name);
        }

        function closeHistoryDrawer() {
            historyDrawer.visible = false;
        }

        function formatTaskTime(isoStr) {
            if (!isoStr) return '—';
            const d = new Date(isoStr);
            if (isNaN(d)) return isoStr;
            return d.toLocaleString();
        }

        // ── Init ──────────────────────────────────────────────────────────
        onMounted(() => {
            setupWebSocket();
            loadConfig();
            loadFiles('');
        });

        return {
            currentView,
            wsConnected,
            currentProvider,
            // chat
            messages,
            chatInput,
            chatFocused,
            chatMessagesEl,
            chatInputEl,
            sendChatMessage,
            quickAction,
            autoResizeTextarea,
            renderMarkdown,
            // terminal
            terminalLines,
            terminalInput,
            terminalOutputEl,
            terminalInputEl,
            clearTerminal,
            executeCommand,
            // files
            fileList,
            currentFilePath,
            breadcrumbs,
            loadFiles,
            onFileClick,
            formatSize,
            // pending modal
            pendingModal,
            confirmPendingCommand,
            cancelPendingCommand,
            // settings
            settingsForm,
            saveStatusMsg,
            saveStatusError,
            saveSettings,
            // tasks
            tasks,
            taskForm,
            taskModal,
            historyDrawer,
            switchToTasks,
            openNewTaskModal,
            openEditTaskModal,
            closeTaskModal,
            saveTask,
            deleteTask,
            toggleTask,
            runTaskNow,
            openTaskHistory,
            closeHistoryDrawer,
            formatTaskTime,
        };
    }
}).mount('#app');
