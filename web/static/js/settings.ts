// PiBot Settings Page

interface ConfigResponse {
    server: { host: string; port: number };
    default_provider: string;
    openai_model: string;
    anthropic_model: string;
    google_model: string;
    ollama_base_url: string;
    ollama_model: string;
    base_directory: string;
}

class Settings {
    constructor() {
        this.init();
    }

    private async init(): Promise<void> {
        await this.loadConfig();
        this.setupEventListeners();
    }

    private async loadConfig(): Promise<void> {
        try {
            const response = await fetch('/api/config');
            const config: ConfigResponse = await response.json();
            
            // Populate form fields
            this.setSelectValue('default-provider', config.default_provider);
            this.setSelectValue('openai-model', config.openai_model);
            this.setSelectValue('anthropic-model', config.anthropic_model);
            this.setSelectValue('google-model', config.google_model);
            this.setInputValue('ollama-url', config.ollama_base_url);
            this.setInputValue('ollama-model', config.ollama_model);
        } catch (error) {
            console.error('Failed to load config:', error);
            this.showStatus('Failed to load configuration', true);
        }
    }

    private setSelectValue(id: string, value: string): void {
        const select = document.getElementById(id) as HTMLSelectElement;
        if (select && value) {
            select.value = value;
        }
    }

    private setInputValue(id: string, value: string): void {
        const input = document.getElementById(id) as HTMLInputElement;
        if (input && value) {
            input.value = value;
        }
    }

    private setupEventListeners(): void {
        const form = document.getElementById('settings-form');
        form?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.saveSettings();
        });
    }

    private async saveSettings(): Promise<void> {
        const formData = {
            default_provider: (document.getElementById('default-provider') as HTMLSelectElement)?.value,
            openai_key: (document.getElementById('openai-key') as HTMLInputElement)?.value || undefined,
            openai_model: (document.getElementById('openai-model') as HTMLSelectElement)?.value,
            anthropic_key: (document.getElementById('anthropic-key') as HTMLInputElement)?.value || undefined,
            anthropic_model: (document.getElementById('anthropic-model') as HTMLSelectElement)?.value,
            google_key: (document.getElementById('google-key') as HTMLInputElement)?.value || undefined,
            google_model: (document.getElementById('google-model') as HTMLSelectElement)?.value,
            ollama_base_url: (document.getElementById('ollama-url') as HTMLInputElement)?.value,
            ollama_model: (document.getElementById('ollama-model') as HTMLInputElement)?.value,
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
                (document.getElementById('openai-key') as HTMLInputElement).value = '';
                (document.getElementById('anthropic-key') as HTMLInputElement).value = '';
                (document.getElementById('google-key') as HTMLInputElement).value = '';
            } else {
                const error = await response.json();
                this.showStatus(`Failed to save: ${error.error}`, true);
            }
        } catch (error) {
            this.showStatus(`Error: ${error}`, true);
        }
    }

    private showStatus(message: string, isError: boolean): void {
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
