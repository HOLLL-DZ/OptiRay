'use strict';
document.addEventListener('DOMContentLoaded', () => {
    let revision = 0;
    async function updateRoutingHealth() {
        const modal = document.getElementById('mt_modal');
        const notice = document.getElementById('mt_runtime_notice');
        if (!notice || !modal || getComputedStyle(modal).display === 'none' || document.hidden) return;
        const current = ++revision;
        const engine = modal.dataset.ruleEngine || 'optirout';
        try {
            if (engine === 'xray') {
                const response = await fetch('/api/xray_routing_config', {cache:'no-store'});
                if (!response.ok) throw new Error('Статус Xray недоступен');
                const data = await response.json();
                if (current !== revision || modal.dataset.ruleEngine !== engine) return;
                notice.classList.remove('routing-health-error');
                notice.textContent = data.routing?.enabled ? 'Правила маршрутизации Xray включены' : 'Правила маршрутизации Xray выключены';
                return;
            }
            const response = await fetch('/api/optirout_status', {cache:'no-store'});
            if (!response.ok) throw new Error('Статус маршрутизации недоступен');
            const status = await response.json();
            if (current !== revision || (modal.dataset.ruleEngine || "optirout") !== engine) return;
            notice.classList.toggle('routing-health-error', status.running && !status.ready);
            notice.textContent = !status.running ? 'OptiRout остановлен' : status.ready ?
                'Маршрутизация работает' + (status.policy_guard?.source === 'cli' ? ' · политики получены через CLI' : '') :
                'Маршрутизация приостановлена: ' + (status.error || 'проверка политик не завершена');
        } catch (error) { if (current !== revision) return; notice.textContent = error.message; notice.classList.add('routing-health-error'); }
    }
    const modal = document.getElementById('mt_modal');
    if (modal) new MutationObserver(updateRoutingHealth).observe(modal, {attributes:true,attributeFilter:['data-rule-engine','style']});
    setInterval(updateRoutingHealth, 5000);
});
