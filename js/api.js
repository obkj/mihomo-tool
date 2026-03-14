export async function getKernelStatus() {
    const response = await fetch('/api/kernel/status');
    return response.json();
}

export async function getTestStatus() {
    const response = await fetch('/api/test/status');
    return response.json();
}

export async function getSettings() {
    const response = await fetch('/api/settings');
    return response.json();
}

export async function saveSettings(settings) {
    const response = await fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings)
    });
    return response.json();
}

export async function triggerSubscriptionUpdate() {
    const response = await fetch('/api/subscription/update', { method: 'POST' });
    return response.json();
}

export async function applyConfig(yaml) {
    const response = await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ yaml })
    });
    return response.json();
}

export async function installKernel() {
    const response = await fetch('/api/kernel/install', { method: 'POST' });
    return response.json();
}

export async function startService() {
    const response = await fetch('/api/service/start', { method: 'POST' });
    return response.json();
}

export async function stopService() {
    const response = await fetch('/api/service/stop', { method: 'POST' });
    return response.json();
}

export async function restartService() {
    const response = await fetch('/api/restart', { method: 'POST' });
    return response;
}

export async function getLogs() {
    const response = await fetch('/api/logs');
    return response.text();
}

export async function getRawConfig() {
    const response = await fetch('/api/config/raw');
    return response.text();
}

export async function getDetailedProxies() {
    const response = await fetch('/api/proxies/detailed');
    return response.json();
}
