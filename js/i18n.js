import * as api from './api.js';

export const i18n = {
    zh: {
        title: 'Mihomo 自动转发管理',
        frontConfig: '前置节点配置',
        subscription: '订阅链接',
        manual: '手动输入',
        subUrl: '订阅地址',
        interval: '自动更新周期（分钟）',
        updateNow: '手动测速',
        manualUrl: '单节点链接 (ss/vmess...)',
        saveSettings: '保存设置',
        landingConfig: '落地节点 (目标)',
        landingUrl: '落地节点链接',
        configPreview: '配置预览',
        preview: '预览配置',
        apply: '应用并运行',
        copy: '复制',
        download: '下载',
        statusInfo: '状态信息',
        kernelStatus: '核心状态',
        install: '更新核心',
        lastUpdate: '最后更新',
        bestNode: '最优节点',
        speed: '最优速度',
        remainingTraffic: '剩余流量',
        restartService: '强制重启核心',
        logs: '控制台日志',
        testing: '正在测试...',
        testingPhase: (phase, curr, total) => phase === 'latency' ? `测试延迟: ${curr}/${total}` : `测试速度: ${curr}/${total}`,
        success: '操作成功',
        error: '发生错误',
        proxyUrl: '内核下载代理',
        start: '启动',
        stop: '停止',
        reverseLogs: '倒序滚动',
        useFallback: '故障转移模式 (Fallback)',
        nodeList: '测速详情',
        nodeName: '节点名称',
        latency: '延迟',
        speed: '平均速度',
        sortByLatency: '按延迟',
        sortBySpeed: '按速度'
    },
    en: {
        title: 'Mihomo Relay Manager',
        frontConfig: 'Front Proxy Config',
        subscription: 'Subscription',
        manual: 'Manual',
        subUrl: 'Subscription URL',
        interval: 'Interval（minutes）',
        updateNow: 'Test Now',
        manualUrl: 'Manual Proxy Link',
        saveSettings: 'Save Settings',
        landingConfig: 'Landing Proxy',
        landingUrl: 'Landing URL',
        configPreview: 'Config Preview',
        preview: 'Preview',
        apply: 'Apply',
        copy: 'Copy',
        download: 'Download',
        statusInfo: 'Status & Info',
        kernelStatus: 'Kernel Status',
        install: 'Update Bin',
        lastUpdate: 'Last Update',
        bestNode: 'Best Node',
        speed: 'Best Speed',
        remainingTraffic: 'Remaining',
        restartService: 'Restart Service',
        logs: 'Backend Logs',
        testing: 'Testing...',
        testingPhase: (phase, curr, total) => phase === 'latency' ? `Latency: ${curr}/${total}` : `Speed: ${curr}/${total}`,
        success: 'Success',
        error: 'Error',
        proxyUrl: 'Download Proxy',
        start: 'Start',
        stop: 'Stop',
        reverseLogs: 'Reverse Logs',
        useFallback: 'Fallback Mode',
        nodeList: 'Speed Test Results',
        nodeName: 'Node Name',
        latency: 'Latency',
        speed: 'Avg Speed',
        sortByLatency: 'By Latency',
        sortBySpeed: 'By Speed'
    }
};

let currentLang = localStorage.getItem('lang') || 'zh';

export function getCurrentLang() {
    return currentLang;
}

export function setCurrentLang(lang) {
    currentLang = lang;
    localStorage.setItem('lang', lang);
}

export function updateUIStrings() {
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        el.textContent = i18n[currentLang][key] || key;
    });
    const langBtn = document.getElementById('langBtn');
    if (langBtn) langBtn.textContent = currentLang === 'zh' ? 'EN' : '中文';
    document.documentElement.lang = currentLang === 'zh' ? 'zh-CN' : 'en';
}

export function toggleLanguage() {
    const nextLang = currentLang === 'zh' ? 'en' : 'zh';
    setCurrentLang(nextLang);
    updateUIStrings();
    api.saveSettings({ language: nextLang }).catch(console.error);
}
