import * as api from './api.js';
import * as i18n from './i18n.js';
import * as utils from './utils.js';
import { generateYAMLConfig } from './config-generator.js';

// No longer needed: let isEditing = false;
let nodeResults = [];
let currentSort = { key: 'latency', desc: false };
let lastTestActive = false;

async function checkStatus() {
    try {
        const data = await api.getKernelStatus();
        const badge = document.getElementById('kernelStatusBadge');
        const toggleBtn = document.getElementById('toggleBtn');
        const currentLang = i18n.getCurrentLang();

        badge.textContent = data.status.toUpperCase();
        badge.className = 'status-badge ' + (data.status === 'Running' ? 'status-running' : 'status-stopped');

        if (data.status === 'Running') {
            toggleBtn.textContent = i18n.i18n[currentLang].stop;
            toggleBtn.style.background = 'var(--danger)';
            toggleBtn.style.display = 'block';
        } else if (data.status === 'Installed') {
            toggleBtn.textContent = i18n.i18n[currentLang].start;
            toggleBtn.style.background = 'var(--primary)';
            toggleBtn.style.display = 'block';
        } else {
            toggleBtn.style.display = 'none';
        }

        const testData = await api.getTestStatus();
        const testStatus = document.getElementById('testStatus');
        const testStatusText = document.getElementById('testStatusText');
        if (testData.is_active && testStatus && testStatusText) {
            testStatus.style.display = 'flex';
            const phase = i18n.i18n[currentLang].testingPhase(testData.phase, testData.current, testData.total);
            testStatusText.textContent = `${phase} | ${testData.node_name}`;
        } else if (testStatus) {
            testStatus.style.display = 'none';
        }

        // Auto-refresh config preview when test finishes
        if (lastTestActive && !testData.is_active) {
            console.log('Test finished, auto-refreshing preview...');
            if (window.generateConfig) window.generateConfig();
        }
        lastTestActive = !!testData.is_active;
    } catch (e) { }
}

async function loadConfig() {
    try {
        const data = await api.getSettings();
        const useSub = data.use_subscription !== false;
        const subRadio = document.querySelector(`input[name="proxySource"][value="${useSub ? 'sub' : 'manual'}"]`);
        if (subRadio) subRadio.checked = true;

        handleSourceChange(useSub ? 'sub' : 'manual');

        document.getElementById('subUrl').value = data.subscription_url || '';
        document.getElementById('subInterval').value = data.interval || 60;
        document.getElementById('manualFrontUrl').value = data.manual_front_proxy || '';
        document.getElementById('landingUrl').value = data.landing_proxy || '';
        document.getElementById('proxyUrl').value = data.download_mirror || 'https://gh-proxy.org/';
        document.getElementById('useFallback').checked = !!data.use_fallback;

        document.getElementById('lastUpdate').textContent = data.last_update || '-';
        document.getElementById('bestProxy').textContent = data.best_proxy_name || '-';
        document.getElementById('bestSpeed').textContent = data.best_proxy_speed || '-';
    } catch (e) { }
}

async function updateStatus() {
    try {
        const data = await api.getSettings();
        document.getElementById('lastUpdate').textContent = data.last_update || '-';
        document.getElementById('bestProxy').textContent = data.best_proxy_name || '-';
        document.getElementById('bestSpeed').textContent = data.best_proxy_speed || '-';
        
        await fetchNodeResults();
    } catch (e) { }
}

async function fetchNodeResults() {
    try {
        const data = await api.getDetailedProxies();
        if (data && data.length > 0) {
            nodeResults = data;
            document.getElementById('nodeListCard').style.display = 'block';
            renderNodeList();
        } else {
            document.getElementById('nodeListCard').style.display = 'none';
        }
    } catch (e) { }
}

function renderNodeList() {
    const listBody = document.getElementById('nodeListBody');
    if (!listBody) return;

    // Apply sorting
    const sorted = [...nodeResults].sort((a, b) => {
        let valA = a[currentSort.key];
        let valB = b[currentSort.key];

        if (currentSort.key === 'speed') {
            // Parse speed: "5.21 MB/s" -> 5.21
            valA = parseSpeedValue(valA);
            valB = parseSpeedValue(valB);
        }

        if (valA < valB) return currentSort.desc ? 1 : -1;
        if (valA > valB) return currentSort.desc ? -1 : 1;
        return 0;
    });

    listBody.innerHTML = sorted.map(node => `
        <tr>
            <td>${node.name}</td>
            <td class="latency-value">${node.latency}ms</td>
            <td class="speed-value">${node.speed || '-'}</td>
        </tr>
    `).join('');
}

function parseSpeedValue(s) {
    if (!s || s === 'N/A' || s === '-') return 0;
    const match = s.match(/(\d+(\.\d+)?)\s*(\w+)\/s/);
    if (!match) return 0;
    const value = parseFloat(match[1]);
    const unit = match[3].toUpperCase();
    if (unit === 'KB') return value * 1024;
    if (unit === 'MB') return value * 1024 * 1024;
    if (unit === 'GB') return value * 1024 * 1024 * 1024;
    return value;
}

async function handleSaveSettings() {
    const settings = {
        use_subscription: document.querySelector('input[name="proxySource"]:checked').value === 'sub',
        subscription_url: document.getElementById('subUrl').value,
        manual_front_proxy: document.getElementById('manualFrontUrl').value,
        interval: parseInt(document.getElementById('subInterval').value) || 60,
        landing_proxy: document.getElementById('landingUrl').value,
        download_mirror: document.getElementById('proxyUrl').value,
        use_fallback: document.getElementById('useFallback').checked,
        language: i18n.getCurrentLang()
    };
    try {
        const result = await api.saveSettings(settings);
        utils.showToast(result.message, 'success');
        await loadConfig();
    } catch (e) {
        utils.showToast('Error', 'error');
    }
}

async function handleTriggerUpdate() {
    try {
        await api.triggerSubscriptionUpdate();
        utils.showToast(i18n.i18n[i18n.getCurrentLang()].testing, 'info');
    } catch (e) { }
}

async function handleApplyToServer() {
    const yaml = document.getElementById('output').textContent;
    if (yaml.startsWith('#')) return utils.showToast(i18n.i18n[i18n.getCurrentLang()].preview, 'info');
    try {
        const result = await api.applyConfig(yaml);
        utils.showToast(result.message, 'success');
    } catch (e) {
        utils.showToast('Error', 'error');
    }
}

async function handleInstallKernel() {
    try {
        await api.installKernel();
        utils.showToast(i18n.i18n[i18n.getCurrentLang()].success, 'success');
    } catch (e) { }
}

async function handleToggleService() {
    const status = document.getElementById('kernelStatusBadge').textContent;
    try {
        const res = status === 'RUNNING' ? await api.stopService() : await api.startService();
        utils.showToast(res.message, 'success');
        setTimeout(checkStatus, 500);
    } catch (e) { }
}

function handleRestartService() {
    api.restartService().then(() => utils.showToast('Restarted', 'success'));
}

function handleSourceChange(type) {
    const subSettings = document.getElementById('subSettings');
    const manualSettings = document.getElementById('manualSettings');
    if (subSettings) subSettings.style.display = type === 'sub' ? 'block' : 'none';
    if (manualSettings) manualSettings.style.display = type === 'manual' ? 'block' : 'none';
}

async function fetchLogs() {
    try {
        const text = await api.getLogs();
        const logsEl = document.getElementById('logs');
        if (logsEl && logsEl.dataset.fullText !== text) {
            logsEl.dataset.fullText = text;
            const isReverse = document.getElementById('reverseLogsToggle').checked;
            if (isReverse) {
                const lines = text.trim().split('\n');
                logsEl.textContent = lines.reverse().join('\n');
                logsEl.parentElement.scrollTop = 0;
            } else {
                logsEl.textContent = text;
                logsEl.parentElement.scrollTop = logsEl.parentElement.scrollHeight;
            }
        }
    } catch (e) { }
}

// Event Listeners and Initialization
document.addEventListener('DOMContentLoaded', () => {
    i18n.updateUIStrings();

    // Global functions for HTML access
    window.toggleLanguage = i18n.toggleLanguage;
    window.handleSourceChange = handleSourceChange;
    window.triggerUpdate = handleTriggerUpdate;
    window.saveSettings = handleSaveSettings;
    window.generateConfig = async () => {
        const useSub = document.querySelector('input[name="proxySource"]:checked').value === 'sub';
        const output = document.getElementById('output');
        if (!output) return;

        if (useSub) {
            try {
                const rawConfig = await api.getRawConfig();
                output.textContent = rawConfig;
                return;
            } catch (e) {
                // Fallback to local generator if server fetch fails
            }
        }
        
        const yaml = generateYAMLConfig();
        if (yaml) {
            output.textContent = yaml;
        }
    };
    window.applyToServer = handleApplyToServer;
    window.copyToClipboard = () => {
        const output = document.getElementById('output');
        if (output && !output.textContent.startsWith('#')) {
            utils.copyTextToClipboard(output.textContent)
                .then(() => utils.showToast(i18n.i18n[i18n.getCurrentLang()].success, 'success'))
                .catch(err => {
                    console.error('Copy failed:', err);
                    utils.showToast('Copy failed', 'error');
                });
        } else {
            utils.showToast(i18n.i18n[i18n.getCurrentLang()].preview, 'info');
        }
    };
    window.exportFile = () => {
        const output = document.getElementById('output');
        if (output) {
            const blob = new Blob([output.textContent], { type: 'text/yaml' });
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'config.yaml';
            a.click();
        }
    };
    window.installKernel = handleInstallKernel;
    window.toggleService = handleToggleService;
    window.restartService = handleRestartService;
    window.sortNodes = (key) => {
        if (currentSort.key === key) {
            currentSort.desc = !currentSort.desc;
        } else {
            currentSort.key = key;
            currentSort.desc = key === 'speed'; // Speed usually desc, latency asc
        }
        renderNodeList();
    };

    // Monitoring
    document.querySelectorAll('input, textarea').forEach(el => {
        el.addEventListener('focus', () => {
            el.select();
        });
    });

    setInterval(checkStatus, 2000);
    setInterval(updateStatus, 5000);
    setInterval(fetchLogs, 2000);
    const logToggle = document.getElementById('reverseLogsToggle');
    if (logToggle) logToggle.addEventListener('change', () => {
        const logsEl = document.getElementById('logs');
        if (logsEl) logsEl.dataset.fullText = ''; // Force refresh
        fetchLogs();
    });

    checkStatus();
    loadConfig();
    fetchLogs();
});
