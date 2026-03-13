// PiBot Settings Page (Compiled from TypeScript)
"use strict";

class Settings {
    constructor() {
        this.init();
    }

    async init() {
        await this.loadConfig();
        this.setupEventListeners();
    }

    async loadConfig() {
        try {
            const response = await fetch('/api/config');
            const config = await response.json();
            
            // Populate form fields
            this.setSelectValue('default-provider', config.default_provider);
            this.setSelectValue('openai-model', config.openai_model);
            this.setSelectValue('anthropic-model', config.anthropic_model);
            this.setSelectValue('google-model', config.google_model);
            this.setInputValue('ollama-url', config.ollama_base_url);
            this.setInputValue('ollama-model', config.ollama_model);
            this.setInputValue('qwen-url', config.qwen_base_url);
            this.setSelectValue('qwen-model', config.qwen_model);
        } catch (error) {
            console.error('Failed to load config:', error);
            this.showStatus('Failed to load configuration', true);
        }
    }

    setSelectValue(id, value) {
        const select = document.getElementById(id);
        if (select && value) {
            select.value = value;
        }
    }

    setInputValue(id, value) {
        const input = document.getElementById(id);
        if (input && value) {
            input.value = value;
        }
    }

    setupEventListeners() {
        const form = document.getElementById('settings-form');
        if (form) {
            form.addEventListener('submit', (e) => {
                e.preventDefault();
                this.saveSettings();
            });
        }
    }

    async saveSettings() {
        const formData = {
            default_provider: document.getElementById('default-provider')?.value,
            openai_key: document.getElementById('openai-key')?.value || undefined,
            openai_model: document.getElementById('openai-model')?.value,
            anthropic_key: document.getElementById('anthropic-key')?.value || undefined,
            anthropic_model: document.getElementById('anthropic-model')?.value,
            google_key: document.getElementById('google-key')?.value || undefined,
            google_model: document.getElementById('google-model')?.value,
            ollama_base_url: document.getElementById('ollama-url')?.value,
            ollama_model: document.getElementById('ollama-model')?.value,
            qwen_key: document.getElementById('qwen-key')?.value || undefined,
            qwen_base_url: document.getElementById('qwen-url')?.value,
            qwen_model: document.getElementById('qwen-model')?.value,
        };

        // Remove undefined/empty values
        const cleanedData = Object.fromEntries(
            Object.entries(formData).filter(([_, v]) => v !== undefined && v !== '')
        );

        try {
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(cleanedData)
            });

            if (response.ok) {
                this.showStatus('Settings saved successfully!', false);
                // Clear password fields after saving
                const openaiKey = document.getElementById('openai-key');
                const anthropicKey = document.getElementById('anthropic-key');
                const googleKey = document.getElementById('google-key');
                const qwenKey = document.getElementById('qwen-key');
                if (openaiKey) openaiKey.value = '';
                if (anthropicKey) anthropicKey.value = '';
                if (googleKey) googleKey.value = '';
                if (qwenKey) qwenKey.value = '';
            } else {
                const error = await response.json();
                this.showStatus(`Failed to save: ${error.error}`, true);
            }
        } catch (error) {
            this.showStatus(`Error: ${error}`, true);
        }
    }

    showStatus(message, isError) {
        const statusEl = document.getElementById('save-status');
        if (statusEl) {
            statusEl.textContent = message;
            statusEl.style.color = isError ? 'var(--accent-danger)' : 'var(--accent-secondary)';
            
            // Clear after 5 seconds
            setTimeout(() => {
                statusEl.textContent = '';
            }, 5000);
        }
    }
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    new Settings();
});
