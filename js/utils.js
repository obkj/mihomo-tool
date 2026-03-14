export function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    if (!container) return;
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateX(100%)';
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

export function copyTextToClipboard(text) {
    if (!text) return Promise.reject('No text to copy');
    
    // Normal navigator.clipboard
    if (navigator.clipboard && window.isSecureContext) {
        return navigator.clipboard.writeText(text);
    }

    // Fallback for non-secure contexts
    return new Promise((resolve, reject) => {
        const textArea = document.createElement("textarea");
        textArea.value = text;
        textArea.style.position = "fixed";
        textArea.style.left = "-9999px";
        textArea.style.top = "0";
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();
        try {
            const successful = document.execCommand('copy');
            document.body.removeChild(textArea);
            if (successful) resolve();
            else reject('Fallback copy failed');
        } catch (err) {
            document.body.removeChild(textArea);
            reject(err);
        }
    });
}

export function safeAtob(str) {
    try {
        str = str.replace(/-/g, '+').replace(/_/g, '/');
        while (str.length % 4) str += '=';
        return decodeURIComponent(escape(atob(str)));
    } catch (e) {
        return atob(str);
    }
}

export function parseUrl(urlStr, name) {
    urlStr = urlStr.trim();
    if (!urlStr) return null;
    try {
        if (urlStr.startsWith('vmess://')) {
            const json = JSON.parse(safeAtob(urlStr.slice(8)));
            const base = {
                name,
                type: 'vmess',
                server: json.add,
                port: parseInt(json.port),
                uuid: json.id,
                alterId: parseInt(json.aid) || 0,
                cipher: json.scy || 'auto',
                tls: json.tls === 'tls' || json.tls === true,
                network: json.net || 'tcp',
                udp: true
            };
            if (base.network === 'ws') {
                base['ws-opts'] = { path: json.path || '/', headers: { Host: json.host || '' } };
            } else if (base.network === 'h2') {
                base['h2-opts'] = { path: json.path || '/', host: [json.host || ''] };
            } else if (base.network === 'grpc') {
                base['grpc-opts'] = { 'grpc-service-name': json.path || '' };
            } else if (base.network === 'tcp' && json.type && json.type !== 'none') {
                base['tcp-opts'] = { header: { type: json.type } };
            }

            if (json.sni) base.servername = json.sni;
            if (json.skipCertVerify === true) base['skip-cert-verify'] = true;
            
            if (!base.server || isNaN(base.port)) return null;
            return base;
        }

        const urlObj = new URL(urlStr);
        const type = urlObj.protocol.replace(':', '');
        const params = urlObj.searchParams;
        let base = { name, type, server: urlObj.hostname, port: parseInt(urlObj.port) || 443, udp: true };
        
        if (['vless', 'trojan', 'ss'].includes(type)) {
            base.uuid = urlObj.username;
            if (type === 'ss' && urlObj.username) {
                try {
                    const info = safeAtob(urlObj.username).split(':');
                    if (info.length >= 2) {
                        base.cipher = info[0];
                        base.password = info[1];
                        delete base.uuid;
                    }
                    const plugin = params.get('plugin');
                    if (plugin) {
                        const parts = plugin.split(';');
                        base.plugin = parts[0];
                        if (parts.length > 1) {
                            base['plugin-opts'] = {};
                            parts.slice(1).forEach(part => {
                                const kv = part.split('=');
                                if (kv.length === 2) base['plugin-opts'][kv[0]] = kv[1];
                            });
                        }
                    }
                } catch (e) {
                    // Not base64 or already decoded
                }
            }
            if (type === 'trojan') { base.password = urlObj.username; delete base.uuid; }
        }

        if (type === 'vless' || type === 'trojan') {
            if (params.get('sni')) base.servername = params.get('sni');
            if (params.get('security') === 'tls' || params.get('security') === 'reality' || params.get('tls')) {
                base.tls = true;
                if (params.get('security') === 'reality') {
                    base.reality = true;
                    base.pbk = params.get('pbk');
                    base.sid = params.get('sid');
                }
            }
            if (params.get('allowInsecure') === '1' || params.get('skip-cert-verify') === 'true') {
                base['skip-cert-verify'] = true;
            }
            if (params.get('alpn')) base.alpn = params.get('alpn').split(',');
            if (params.get('flow')) base.flow = params.get('flow');
            if (params.get('fp')) base.fp = params.get('fp');
            if (params.get('type')) base.network = params.get('type');
            if (base.network === 'ws') {
                base['ws-opts'] = {
                    path: params.get('path') || '/',
                    headers: { Host: params.get('host') || base.server }
                };
            } else if (base.network === 'grpc') {
                base['grpc-opts'] = { 'grpc-service-name': params.get('serviceName') || '' };
            } else if (base.network === 'h2') {
                base['h2-opts'] = {
                    path: params.get('path') || '/',
                    host: [params.get('host') || base.server]
                };
            }
            if (params.get('encryption')) base.cipher = params.get('encryption');
        }

        if (!base.server || isNaN(base.port)) return null;
        return base;
    } catch (e) {
        console.error('Parse error:', e);
        return null;
    }
}
