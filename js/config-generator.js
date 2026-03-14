import { parseUrl } from './utils.js';
import { i18n, getCurrentLang } from './i18n.js';
import { showToast } from './utils.js';

export function generateYAMLConfig() {
    const useSub = document.querySelector('input[name="proxySource"]:checked').value === 'sub';
    let frontProxy;
    const currentLang = getCurrentLang();

    if (useSub) {
        frontProxy = { name: 'proxy-front', type: 'relay-source', server: 'dynamic-from-sub', port: 443, uuid: 'dynamic' };
    } else {
        frontProxy = parseUrl(document.getElementById('manualFrontUrl').value, 'proxy-front');
    }

    const landingProxy = parseUrl(document.getElementById('landingUrl').value, 'proxy-landing');

    if (!frontProxy || !landingProxy) {
        showToast(i18n[currentLang].error, 'error');
        return null;
    }

    // Helper for common proxy fields
    const serializeProxy = (p) => {
        let lines = `  - name: "${p.name}"\n    type: ${p.type}\n    server: ${p.server}\n    port: ${p.port}\n`;
        if (p.uuid) lines += `    uuid: ${p.uuid}\n`;
        if (p.password) lines += `    password: ${p.password}\n`;
        if (p.type === 'vmess') lines += `    alterId: ${p.alterId || 0}\n`;
        if (p.cipher) lines += `    cipher: ${p.cipher}\n`;
        if (p.udp) lines += `    udp: true\n`;
        if (p.tls) lines += `    tls: true\n`;
        if (p['skip-cert-verify']) lines += `    skip-cert-verify: true\n`;
        if (p.servername) lines += `    servername: ${p.servername}\n`;
        if (p.alpn) lines += `    alpn:\n      - ${p.alpn.join('\n      - ')}\n`;
        if (p.flow) lines += `    flow: ${p.flow}\n`;
        if (p.network) lines += `    network: ${p.network}\n`;
        if (p.fp) lines += `    client-fingerprint: ${p.fp}\n`;
        if (p.plugin) {
            lines += `    plugin: ${p.plugin}\n`;
            if (p['plugin-opts']) {
                lines += `    plugin-opts:\n`;
                for (const [key, value] of Object.entries(p['plugin-opts'])) {
                    lines += `      ${key}: ${value}\n`;
                }
            }
        }
        if (p.reality) {
            lines += `    reality-opts:\n      public-key: ${p.pbk}\n`;
            if (p.sid) lines += `      short-id: ${p.sid}\n`;
        }
        if (p['ws-opts']) {
            lines += `    ws-opts:\n      path: "${p['ws-opts'].path}"\n      headers:\n        Host: ${p['ws-opts'].headers.Host}\n`;
        }
        if (p['h2-opts']) {
            lines += `    h2-opts:\n      host:\n        - ${p['h2-opts'].host.join('\n        - ')}\n      path: "${p['h2-opts'].path}"\n`;
        }
        if (p['grpc-opts']) {
            lines += `    grpc-opts:\n      grpc-service-name: "${p['grpc-opts']['grpc-service-name']}"\n`;
        }
        if (p['tcp-opts'] && p['tcp-opts'].header) {
            lines += `    tcp-opts:\n      header:\n        type: ${p['tcp-opts'].header.type}\n`;
        }
        return lines;
    };

    let yaml = `socks-port: 7891\nport: 7890\nallow-lan: true\nlog-level: info\nexternal-controller: :9090\n\nproxies:\n`;
    
    // Front proxy
    yaml += serializeProxy(frontProxy) + '\n';

    // Landing proxy with dialer-proxy
    let landingLines = serializeProxy(landingProxy);
    landingLines += `    dialer-proxy: "proxy-front"\n`;
    yaml += landingLines + '\n';

    yaml += `proxy-groups:\n  - name: "Relay-Chain"\n    type: select\n    proxies:\n      - "proxy-landing"\n\nrules:\n  - MATCH,Relay-Chain`;
    
    return yaml;
}
