'use strict';
document.addEventListener('DOMContentLoaded', () => {
    async function updateRoutingHealth() {
        const modal = document.getElementById('mt_modal');
        const notice = document.getElementById('mt_runtime_notice');
        if (!notice || !modal || getComputedStyle(modal).display === 'none' || document.hidden) return;
        try {
            const response = await fetch('/api/optirout_status', {cache:'no-store'});
            if (!response.ok) throw new Error('Статус маршрутизации недоступен');
            const status = await response.json();
            notice.classList.toggle('routing-health-error', status.running && !status.ready);
            notice.textContent = !status.running ? 'OptiRout остановлен' : status.ready ?
                'Маршрутизация работает' + (status.policy_guard?.source === 'cli' ? ' · политики получены через CLI' : '') :
                'Маршрутизация приостановлена: ' + (status.error || 'проверка политик не завершена');
        } catch (error) { notice.textContent = error.message; notice.classList.add('routing-health-error'); }
    }
    setInterval(updateRoutingHealth, 5000);
});
