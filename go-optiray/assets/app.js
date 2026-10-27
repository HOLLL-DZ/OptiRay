
/* ==== OPTIRAY_CLEAN_BLOCK_A (top of JS) ==== */
(function() {
    'use strict';

    /* 1. Фильтр ложных alert'ов (страховка на случай, если где-то остался) */
    var _origAlert = window.alert;
    window.alert = function(msg) {
        var m = String(msg == null ? '' : msg);
        var silent = [
            'туннель ещё прогревается',
            'Сервер не отвечает.\n\nВыберите другой сервер',
            'Возможно он заблокирован'
        ];
        for (var i = 0; i < silent.length; i++) {
            if (m.indexOf(silent[i]) >= 0) {
                if (window.console && console.log) {
                    console.log('optiray (suppressed alert):', m);
                }
                return;
            }
        }
        return _origAlert.apply(window, arguments);
    };

    /* 2. Test-кнопки работают как обычные <a target="_blank">.
       Capture-фаза, stopImmediatePropagation, БЕЗ preventDefault —
       браузер сам откроет href в новой вкладке.
       Этот обработчик присоединяется ПЕРВЫМ (Block A в начале JS),
       поэтому в capture-фазе выполняется раньше всех остальных. */
    document.addEventListener('click', function(e) {
        var el = e.target && e.target.closest
            ? e.target.closest('.btn-test-site, .btn-test-float')
            : null;
        if (!el) return;
        e.stopImmediatePropagation();
        /* preventDefault НЕ вызываем — ссылка откроется штатно */
    }, true);
})();

// Флаг: если основной источник (jsdelivr) не ответил — пробуем flagcdn.
// Если и он упал — тихо скрываем <img>, не оставляя пустой рамки.
function flagFallback(img) {
    if (!img.dataset.tried) {
        img.dataset.tried = '1';
        img.src = img.dataset.fallback;
    } else {
        img.style.display = 'none';
    }
}
var subscriptionRefreshBusy = false;

document.addEventListener('keydown', function(event) {
    var input = event.target;
    if (event.key !== 'Enter' || event.isComposing || !input.matches ||
        !input.matches('form[action="/manage"] .sub-url-input')) return;
    // Prevent implicit form submission from activating Add/Delete instead.
    event.preventDefault();
    if (event.repeat || subscriptionRefreshBusy || !input.value.trim()) return;
    var button = input.form.querySelector('.btn-refresh-icon');
    if (button) refreshSubsIcon(button);
});

async function refreshSubsIcon(btn) {
    if (!btn || btn.disabled || subscriptionRefreshBusy) return;
    try { clearAllPingCache(); } catch(e) {}
    try { stopPing(); } catch(e) {}
    var form = document.querySelector('form[action="/manage"]');
    if (!form) { alert('Форма не найдена'); return; }
    subscriptionRefreshBusy = true;
    var fd = new FormData(form);
    fd.set('action', 'update');
    var params = new URLSearchParams();
    for (var pair of fd.entries()) {
        if (typeof pair[1] === 'string') params.append(pair[0], pair[1]);
    }
    btn.disabled = true;
    var ov = document.getElementById('update_overlay');
    if (ov) ov.classList.add('active');
    try {
        await fetch('/manage', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: params.toString()
        });
    } catch(e) {}
    await new Promise(function(r) { setTimeout(r, 250); });
    window.location.reload();
}

async function refreshSubs(btn) {
    if (!btn || btn.disabled || subscriptionRefreshBusy) return;
    try { clearAllPingCache(); } catch(e) {}
    try { stopPing(); } catch(e) {}
    var form = document.querySelector('form[action="/manage"]');
    if (!form) { alert('Форма не найдена'); return; }
    subscriptionRefreshBusy = true;
    var fd = new FormData(form);
    fd.set('action', 'update');
    var params = new URLSearchParams();
    for (var pair of fd.entries()) {
        if (typeof pair[1] === 'string') params.append(pair[0], pair[1]);
    }
    var orig = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-sm"></span>Обновление...';
    var ov = document.getElementById('update_overlay');
    if (ov) ov.classList.add('active');
    try {
        await fetch('/manage', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: params.toString()
        });
    } catch(e) {}
    await new Promise(function(r) { setTimeout(r, 250); });
    window.location.reload();
}

function toggleSubsCollapse() {
    var names = document.getElementById('subs_names_container');
    var full = document.getElementById('subs_container');
    var addRow = document.getElementById('subs_add_row');
    var arrow = document.getElementById('subs_arrow');
    if (!names || !full || !arrow) return;
    var isCollapsed = full.style.display === 'none';
    if (isCollapsed) {
        names.style.display = 'none';
        full.style.display = '';
        if (addRow) addRow.style.display = '';
        arrow.innerHTML = serverChevron(false);
        localStorage.setItem('subs_collapsed', '0');
    } else {
        names.style.display = '';
        full.style.display = 'none';
        if (addRow) addRow.style.display = 'none';
        arrow.innerHTML = serverChevron(true);
        localStorage.setItem('subs_collapsed', '1');
    }
}

function applySubsCollapseState() {
    var names = document.getElementById('subs_names_container');
    var full = document.getElementById('subs_container');
    var addRow = document.getElementById('subs_add_row');
    var arrow = document.getElementById('subs_arrow');
    if (!names || !full || !arrow) return;
    if (localStorage.getItem('subs_collapsed') === '1') {
        names.style.display = '';
        full.style.display = 'none';
        if (addRow) addRow.style.display = 'none';
        arrow.innerHTML = serverChevron(true);
    } else {
        names.style.display = 'none';
        full.style.display = '';
        if (addRow) addRow.style.display = '';
        arrow.innerHTML = serverChevron(false);
    }
}

function toggleTheme() {
    var doc = document.documentElement;
    var cur = doc.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
    doc.setAttribute('data-theme', cur);
    localStorage.setItem('optiray_theme', cur);
}

var isPinging = false, abortCtrl = null;

function restorePings() {
    var saved = localStorage.getItem('optiray_ping_cache');
    if (!saved) return;
    try {
        var cache = JSON.parse(saved);
        document.querySelectorAll('.server-card').forEach(function(card) {
            var sid = card.dataset.sid;
            if (sid && cache[sid]) {
                var b = card.querySelector('.ping-badge');
                if (b) {
                    b.innerText = cache[sid].text;
                    b.className = 'ping-badge ' + cache[sid].cls;
                    if (cache[sid].min && cache[sid].max && cache[sid].min > 0) {
                        b.title = cache[sid].text + ' (min ' + cache[sid].min + ' / max ' + cache[sid].max + ')';
                    }
                }
            }
        });
    } catch(e) {}
}

function savePingToCache(sName, text, cls) {
    var cache = {};
    try { cache = JSON.parse(localStorage.getItem('optiray_ping_cache') || '{}'); } catch(e) {}
    cache[sName] = { text: text, cls: cls };
    localStorage.setItem('optiray_ping_cache', JSON.stringify(cache));
}

function clearPings(cards) {
    var cache = {};
    try { cache = JSON.parse(localStorage.getItem('optiray_ping_cache') || '{}'); } catch(e) {}
    cards.forEach(function(card) {
        delete cache[card.dataset.sid];
        var b = card.querySelector('.ping-badge');
        if (b) { b.innerText = '-- ms'; b.className = 'ping-badge'; }
    });
    localStorage.setItem('optiray_ping_cache', JSON.stringify(cache));
}

function clearAllPingCache() { localStorage.removeItem('optiray_ping_cache'); }

function stopPing() {
    isPinging = false;
    if (abortCtrl) abortCtrl.abort();
    Object.keys(pingProgressState).forEach(hidePingProgress);
}
/* ==== МОДАЛКА ОЖИДАНИЯ ПОДКЛЮЧЕНИЯ ==== */
function showWaitModal(title, subtext) {
    var m = document.getElementById('wait_modal');
    var t = document.getElementById('wait_modal_title');
    var s = document.getElementById('wait_modal_subtext');
    if (t) t.innerText = title || 'Подключение...';
    if (s) s.innerText = subtext || '';
    if (m) m.classList.add('active');
}
function updateWaitModal(subtext) {
    var s = document.getElementById('wait_modal_subtext');
    if (s && subtext != null) s.innerText = subtext;
}
function hideWaitModal() {
    var m = document.getElementById('wait_modal');
    if (m) {m.classList.remove('active');if(m.dataset.operation){delete m.dataset.operation;var animation=m.querySelector('.update-spinner');if(animation && animation.dataset.originalMarkup){animation.innerHTML=animation.dataset.originalMarkup;animation.classList.remove('maintenance-animation');}}}
}

async function probeUrl(url, timeoutMs) {
    timeoutMs = timeoutMs || 1500;
    try {
        var ctrl = new AbortController();
        var tid = setTimeout(function() { ctrl.abort(); }, timeoutMs);
        await fetch(url, {
            mode: 'no-cors',
            cache: 'no-store',
            redirect: 'follow',
            referrerPolicy: 'no-referrer',
            signal: ctrl.signal
        });
        clearTimeout(tid);
        return true;
    } catch (e) {
        return false;
    }
}

function _testUrlFromDom() {
    var el = document.getElementById('test_float_btn');
    if (el && el.getAttribute('href')) return el.getAttribute('href');
    el = document.querySelector('.btn-test-site');
    if (el && el.getAttribute('href')) return el.getAttribute('href');
    return 'http://cp.cloudflare.com/generate_204';
}

function openTestUrl(url) {
    if (!url) return;
    window.open(url, '_blank', 'noopener');
}

/* Base click interceptor для Test удалён (Block A) */



async function runPing(card, signal, pingType) {
    pingType = pingType || 'real';
    var b = card.querySelector('.ping-badge');
    b.innerText = '...'; b.className = 'ping-badge';
    try {
        var el = document.getElementById('ping_timeout_input');
        var t = el ? el.value : '3.0';
        var res = await fetch('/api/ping?sid=' + encodeURIComponent(card.dataset.sid) +
                              '&timeout=' + t + '&type=' + pingType, { signal: signal });
        var data = await res.json();
        var txt = 'dead', cls = 'ping-red';
        if (data.latency === -2) {
            txt = 'n/a'; cls = '';
        } else if (data.latency > 0) {
            txt = data.latency + ' ms';
            cls = data.latency < 400 ? 'ping-green' : (data.latency < 800 ? 'ping-orange' : 'ping-red');
        }
        b.innerText = txt;
        if (cls) b.classList.add(cls);
        b.title = '';
        savePingToCache(card.dataset.sid, txt, cls);
    } catch(e) {
        if (e.name === 'AbortError') {
            if (b.innerText === '...') {
                b.innerText = '-- ms';
                b.className = 'ping-badge';
            }
        } else {
            b.innerText = 'err';
            b.classList.add('ping-red');
            savePingToCache(card.dataset.sid, 'err', 'ping-red');
        }
    }
}

async function pingSingle(cardId, pingType) {
    var card = document.getElementById(cardId);
    if (card) await runPing(card, new AbortController().signal, pingType || 'real');
}

/* --- Индикатор прогресса пинга в блоке PING ---
   Показывает «X %» рядом с заголовком PING во время замера.
   После достижения 100% индикатор скрывается через 5 секунд. */
var pingProgressState = {};

function _subIdOfCard(card) {
    if (!card || !card.className) return null;
    var m = String(card.className).match(/(?:^|\s)sub-group-(\d+)(?:\s|$)/);
    return m ? m[1] : null;
}

function showPingProgress(subId, total) {
    var el = document.getElementById('ping_progress_' + subId);
    if (!el || !total) return;
    var st = pingProgressState[subId];
    if (st && st.hideTimer) clearTimeout(st.hideTimer);
    el.textContent = '0%';
    el.classList.add('active');
    return pingProgressState[subId] = { total: total, done: 0, hideTimer: null };
}

function incrementPingProgress(subId, expected) {
    var st = pingProgressState[subId];
    if (!st || (expected && st !== expected)) return;
    st.done++;
    var pct = Math.min(100, Math.round(st.done * 100 / st.total));
    var el = document.getElementById('ping_progress_' + subId);
    if (el) el.textContent = pct + '%';
}

function hidePingProgress(subId) {
    var st = pingProgressState[subId];
    if (st && st.hideTimer) clearTimeout(st.hideTimer);
    var el = document.getElementById('ping_progress_' + subId);
    if (el) { el.classList.remove('active'); el.textContent = ''; }
    delete pingProgressState[subId];
}

function finishPingProgress(subId, expected) {
    var st = pingProgressState[subId];
    if (!st || (expected && st !== expected)) return;
    var el = document.getElementById('ping_progress_' + subId);
    if (el) {
        el.textContent = '100%';
        st.hideTimer = setTimeout(function() {
            if (pingProgressState[subId] === st) hidePingProgress(subId);
        }, 5000);
    } else {
        delete pingProgressState[subId];
    }
}

async function runQueue(cards, concurrency, pingType) {
    pingType = pingType || 'real';
    var queueCtrl = abortCtrl;

    // Параллелизм читаем из настроек (поля всегда в DOM, даже когда модалка закрыта)
    var _tcpEl  = document.getElementById('ping_tcp_conc_input');
    var _realEl = document.getElementById('ping_real_conc_input');
    var _tcpConc  = _tcpEl  ? parseInt(_tcpEl.value, 10)  : 5;
    var _realConc = _realEl ? parseInt(_realEl.value, 10) : 2;
    if (!(_tcpConc  >= 1 && _tcpConc  <= 20)) _tcpConc  = 5;
    if (!(_realConc >= 1 && _realConc <= 10)) _realConc = 2;

    var _qDelayEl = document.getElementById('ping_sample_delay_input');
    var _qDelay = _qDelayEl ? parseInt(_qDelayEl.value, 10) : 300;
    if (!(_qDelay >= 0 && _qDelay <= 5000)) _qDelay = 300;

    // Считаем, сколько карточек в каждой подписке — для расчёта процентов
    var bySub = {};
    cards.forEach(function(c) {
        var sid = _subIdOfCard(c);
        if (sid) {
            if (!bySub[sid]) bySub[sid] = { total: 0, done: 0 };
            bySub[sid].total++;
        }
    });
    Object.keys(bySub).forEach(function(sid) {
        bySub[sid].state = showPingProgress(sid, bySub[sid].total);
    });

    var index = 0;
    var slots = pingType === 'tcp' ? _tcpConc : _realConc;
    var workers = [];
    for (var i = 0; i < slots; i++) {
        workers.push((async function() {
            while (index < cards.length && isPinging && queueCtrl === abortCtrl && !queueCtrl.signal.aborted) {
                var card = cards[index++];
                await runPing(card, queueCtrl.signal, pingType);
                var sid = _subIdOfCard(card);
                if (sid) incrementPingProgress(sid, bySub[sid].state);
                if (_qDelay > 0 && index < cards.length && isPinging) {
                    await new Promise(function(r){ setTimeout(r, _qDelay); });
                }
            }
        })());
    }
    await Promise.all(workers);

    Object.keys(bySub).forEach(function(sid) {
        finishPingProgress(sid, bySub[sid].state);
    });
    if (queueCtrl === abortCtrl) isPinging = false;
}

async function pingGroup(subId, pingType) {
    stopPing();
    var cards = Array.from(document.querySelectorAll('.sub-group-' + subId));
    clearPings(cards);
    isPinging = true;
    abortCtrl = new AbortController();
    await runQueue(cards, 2, pingType);
}

async function pingGroupFavs(subId, pingType) {
    stopPing();
    clearPings(Array.from(document.querySelectorAll('.sub-group-' + subId)));
    var favs = Array.from(document.querySelectorAll('.sub-group-' + subId + '[data-fav="1"]'));
    if (!favs.length) return;
    isPinging = true;
    abortCtrl = new AbortController();
    await runQueue(favs, 2, pingType);
}

async function pingAll(pingType) {
    stopPing();
    var cards = Array.from(document.querySelectorAll('.server-card'));
    clearPings(cards);
    isPinging = true;
    abortCtrl = new AbortController();
    await runQueue(cards, 2, pingType);
}

async function pingAllFavs(pingType) {
    stopPing();
    clearPings(Array.from(document.querySelectorAll('.server-card')));
    var favs = Array.from(document.querySelectorAll('.server-card[data-fav="1"]'));
    if (!favs.length) return;
    isPinging = true;
    abortCtrl = new AbortController();
    await runQueue(favs, 2, pingType);
}

async function connectServer(btn) {
    var card = btn.closest('.server-card');
    var orig = btn.innerText;
    btn.innerText = 'Подключение...';
    btn.disabled = true;
    showWaitModal('Подключение к серверу...', 'Применение конфигурации и прогрев туннеля');
    try {
        var fd = new URLSearchParams();
        fd.append('sid', card.dataset.sid);
        var res = await fetch('/select', { method: 'POST', body: fd });
        var payload = {};
        try { payload = await res.json(); } catch(e) {}
        if (res.ok) {
            document.querySelectorAll('.server-card').forEach(function(c) {
                c.classList.remove('active-card');
                c.dataset.active = '0';
                var s = c.querySelector('.action-slot');
                if (s) s.innerHTML = '<button type="button" onclick="connectServer(this)" class="btn-connect text-style-7">Подключить</button>';
            });
            card.classList.add('active-card');
            card.dataset.active = '1';
            card.parentNode.insertBefore(card, card.parentNode.firstChild);
            card.querySelector('.action-slot').innerHTML = '<div class="badge-connected text-style-7">Подключено</div>';
            var sec = card.closest('[id^="sub-section-"]');
            if (sec) {
                document.getElementById('active-sub-name').innerText = sec.dataset.subname || 'Подписка';
            }
            document.getElementById('active-srv-name').innerText = card.dataset.name;
            var dot = document.getElementById('active-status-dot');
            if (dot) {
                dot.classList.remove('disconnected');
                dot.classList.add('connected');
            }
            var coreBadge = document.getElementById('active-core-badge');
            if (coreBadge) {
                var protoLow = (card.dataset.protocol || '').toLowerCase();
                if (protoLow === 'hysteria' || protoLow === 'hysteria2') {
                    coreBadge.innerText = 'Hysteria';
                    coreBadge.className = 'core-badge core-hysteria';
                } else {
                    coreBadge.innerText = 'Xray';
                    coreBadge.className = 'core-badge core-xray';
                }
                coreBadge.style.display = '';
            }
            updateWaitModal('Прогрев туннеля...');
            var probeTarget = _testUrlFromDom();
            for (var wAttempt = 1; wAttempt <= 12; wAttempt++) {
                if (await probeUrl(probeTarget, 1500)) break;
                updateWaitModal('Прогрев туннеля... (' + wAttempt + '/12)');
                await new Promise(function(r) { setTimeout(r, 300); });
            }
            hideWaitModal();
            if (payload.warning) setTimeout(function() { alert(payload.warning); }, 100);
        } else {
            hideWaitModal();
            alert('Ошибка подключения: ' + (payload.error || 'проверьте ядро'));
            btn.innerText = orig;
            btn.disabled = false;
        }
    } catch(e) {
        hideWaitModal();
        alert('Сетевая ошибка: ' + e.message);
        btn.innerText = orig;
        btn.disabled = false;
    }
}

async function toggleFav(btn, cardId) {
    var card = document.getElementById(cardId);
    if (!card || btn.disabled) return;
    btn.disabled = true;
    var newFav = card.dataset.fav === '1' ? '0' : '1';
    try {
        var res = await fetch('/api/toggle_favorite', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'sid=' + encodeURIComponent(card.dataset.sid)
        });
        if (res.ok) {
            var result = await res.json();
            newFav = result.favorite ? '1' : '0';
            card.dataset.fav = newFav;
            var grid = card.parentNode;
            var sec = card.closest('.sub-section');
            if (newFav === '1') {
                btn.classList.add('active');
                btn.innerHTML = "<svg viewBox='0 0 24 24' width='16' height='16' fill='currentColor' stroke='none' aria-hidden='true'><path d='M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z'/></svg>";

            } else {
                btn.classList.remove('active');
                btn.innerHTML = "<svg viewBox='0 0 24 24' width='16' height='16' fill='none' stroke='currentColor' stroke-width='2' stroke-linecap='round' stroke-linejoin='round' aria-hidden='true'><path d='M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z'/></svg>";

            }
            if (sec) {
                var msg = document.getElementById('no_favs_msg_' + sec.id.split('-')[2]);
                if (sec.classList.contains('collapsed') && msg) {
                    msg.style.display = grid.querySelectorAll('[data-fav="1"], [data-active="1"]').length ? 'none' : 'block';
                }
            }
        }
    } catch(e) {} finally { btn.disabled = false; }
    browseServers();
}

function toggleSubCollapse(subId) {
    var sec = document.getElementById('sub-section-' + subId);
    var btn = document.getElementById('collapse_btn_' + subId);
    if (!sec || !btn) return;
    var isC = sec.classList.toggle('collapsed');
    renderSubscriptionToggle(sec, btn, isC);
    localStorage.setItem('sub_collapsed_' + subId, isC ? '1' : '0');
    var msg = document.getElementById('no_favs_msg_' + subId);
    if (msg) {
        msg.style.display = (isC && !sec.querySelectorAll('[data-fav="1"], [data-active="1"]').length) ? 'block' : 'none';
    }
}

function genNewHwid() {
    var el = document.getElementById('hwid_input');
    el.value = ([1e7] + -1e3 + -4e3 + -8e3 + -1e11).replace(/[018]/g, function(c) {
        return (c ^ crypto.getRandomValues(new Uint8Array(1))[0] & 15 >> c / 4).toString(16);
    });
    el.dispatchEvent(new Event('input',{bubbles:true}));
}

var subColorSaving = false;
async function changeSubColor(subId, hex) {
    if (subColorSaving) return false;
    subColorSaving = true;
    var controls = Array.from(document.querySelectorAll('.sub-manual-color-trigger,.sub-row .dice-btn,#sub_manual_palette input'));
    controls.forEach(function (control) { control.disabled = true; });
    try {
        var response = await fetch('/api/save_sub_color', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'sub_id=' + subId + '&color=' + encodeURIComponent(hex)
        });
        if (!response.ok) throw new Error('Не удалось сохранить цвет');
        var result = await response.json();
        if (result.ok === false) throw new Error(result.error || 'Не удалось сохранить цвет');
        var source = document.querySelector('input[name="sub_color_' + subId + '"]');
        if (source) source.value = hex;
    var row = document.getElementById('sub-row-' + subId);
    var sec = document.getElementById('sub-section-' + subId);
    if (row) {
        row.style.setProperty('--subscription-color',hex);
        row.style.background = hex + '10';
        row.style.border = '1px solid ' + hex + '35';
        row.style.borderLeft = '5px solid ' + hex;
    }
    if (sec) {
        sec.style.setProperty('--subscription-color',hex);
        sec.style.background = hex + '10';
        sec.style.border = '1px solid ' + hex + '35';
        sec.style.borderLeft = '5px solid ' + hex;
        sec.dataset.subcolor = hex;
    }
    var rows = Array.from(document.querySelectorAll('.sub-row'));
    var chip = document.querySelectorAll('.subs-name-chip')[rows.indexOf(row)];
    if (chip) { chip.style.setProperty('--subscription-color',hex); var dot=chip.querySelector('.subs-name-dot'); if(dot)dot.style.background=hex; }
        return true;
    } catch (error) {
        alert(error.message || 'Не удалось сохранить цвет');
        return false;
    } finally {
        subColorSaving = false;
        controls.forEach(function (control) { control.disabled = false; });
    }
}

function randomSubColor(subId) {
    var pal = ["#2b8a3e", "#1971c2", "#d9480f", "#862e9c", "#0c8599", "#e67700", "#495057"];
    var inp = document.querySelector('input[name="sub_color_' + subId + '"]');
    var choices=pal.filter(function(color){return !inp || color.toLowerCase() !== inp.value.toLowerCase();});
    var hex = choices[Math.floor(Math.random() * choices.length)];

    changeSubColor(subId, hex);
}

var xrayVersionsLoading = false;
async function loadXrayVersions(force) {
    if (xrayVersionsLoading) return;
    var s = document.getElementById('xray_installed_badge');
    var sel = document.getElementById('xray_version_select');
    if (!s || !sel) return;
    xrayVersionsLoading = true;
    var infoRevision=window.optiraySettingsInfoRevision||0;
    var previousVersion = sel.value;
    s.innerText = 'Опрос...';
    try {
        var response = await fetch('/api/xray_info' + (force ? '?refresh=1' : ''), {cache:'no-store'});
        if (!response.ok) throw new Error('HTTP ' + response.status);
        var res = await response.json();
        setXraySubscriptionAvailability(res.installed);
        s.innerText = res.installed ? ('Установлено v' + res.version) : 'Не установлено';
        s.className = 'badge-ver ' + (res.installed ? 'installed' : 'missing');
        sel.innerHTML = '';
        res.available.sort(function(a,b){var av=a.version.split(/[.+-]/).map(Number),bv=b.version.split(/[.+-]/).map(Number);for(var n=0;n<3;n++){var difference=(bv[n]||0)-(av[n]||0);if(difference)return difference;}return Number(a.is_pre)-Number(b.is_pre);});
        res.available.forEach(function(i, idx) {
            var o = document.createElement('option');
            o.value = i.version;
            o.textContent = 'v' + i.version + (i.is_stable ? ' (Стабильная)' : (i.is_pre ? ' (Pre)' : ' (Rel)')) + (idx === 0 && res.versions_source !== 'cache' ? ' [New]' : '');
            
            sel.appendChild(o);
        });
        if (sel.options.length) sel.selectedIndex = 0;
        var notice = document.getElementById('xray_versions_notice');
        if (notice) setSettingsInfo(notice,res.versions_error || '',infoRevision);
        if (!res.available.length) {
            var empty = document.createElement('option'); empty.value = ''; empty.textContent = 'Версии недоступны'; empty.disabled = true; sel.appendChild(empty);
        }
    } catch(e) {
        s.innerText = 'Статус недоступен';
        var notice = document.getElementById('xray_versions_notice');
        if (notice) setSettingsInfo(notice,'Не удалось загрузить список версий. Повторите позже.',infoRevision);
    } finally { xrayVersionsLoading = false;if(window.refreshCoreUpdateBadges)window.refreshCoreUpdateBadges(); }
}

async function installXrayVersion(btn) {
    var sel = document.getElementById('xray_version_select');
    var v = sel ? sel.value : '';
    if (!v) return;
    var orig = btn.innerText;
    btn.innerText = 'Установка...';
    btn.disabled = true;
    try {
        var res = await (await fetch('/api/xray_action', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'action=install&version=' + encodeURIComponent(v)
        })).json();
        alert(res.message);
        await loadXrayVersions();
    } catch(e) { alert('Ошибка: ' + e.message); }
    finally { btn.innerText = orig; btn.disabled = false; }
}

async function uninstallXray(btn) {
    if (!confirm('Удалить ядро Xray?')) return;
    var orig = btn.innerText;
    btn.innerText = 'Удаление...';
    btn.disabled = true;
    try {
        var res = await (await fetch('/api/xray_action', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'action=uninstall'
        })).json();
        alert(res.message);
        await loadXrayVersions();
    } catch(e) {}
    finally { btn.innerText = orig; btn.disabled = false; }
}

var hysteriaVersionsLoading=false;
async function checkHysteria(force) {
    if (hysteriaVersionsLoading) return;
    var s = document.getElementById('hysteria_installed_badge');
    var sel = document.getElementById('hysteria_version_select');
    if (!s || !sel) return;
    hysteriaVersionsLoading = true;
    var infoRevision=window.optiraySettingsInfoRevision||0;
    var previousVersion = sel.value;
    s.innerText = 'Опрос...';
    try {
        var response = await fetch('/api/hysteria_info' + (force ? '?refresh=1' : ''), {cache:'no-store'});
        if (!response.ok) throw new Error('HTTP ' + response.status);
        var res = await response.json();
        s.innerText = res.installed ? ('Установлено v' + res.version) : 'Не установлено';
        s.className = 'badge-ver ' + (res.installed ? 'installed' : 'missing');
        sel.innerHTML = '';
        res.available.sort(function(a,b){var av=a.version.split(/[.+-]/).map(Number),bv=b.version.split(/[.+-]/).map(Number);for(var n=0;n<3;n++){var difference=(bv[n]||0)-(av[n]||0);if(difference)return difference;}return Number(a.is_pre)-Number(b.is_pre);});
        res.available.forEach(function(i, idx) {
            var o = document.createElement('option');
            o.value = i.version;
            o.textContent = 'v' + i.version + (i.is_stable ? ' (Стабильная)' : (i.is_pre ? ' (Pre)' : ' (Rel)')) + (idx === 0 && res.versions_source !== 'cache' ? ' [New]' : '');
            
            sel.appendChild(o);
        });
        if (sel.options.length) sel.selectedIndex = 0;
        var notice = document.getElementById('hysteria_versions_notice');
        if (notice) setSettingsInfo(notice,res.versions_error || '',infoRevision);
        if (!res.available.length) {
            var empty = document.createElement('option'); empty.value = ''; empty.textContent = 'Версии недоступны'; empty.disabled = true; sel.appendChild(empty);
        }
    } catch(e) {
        s.innerText = 'Статус недоступен';
        var notice = document.getElementById('hysteria_versions_notice');
        if (notice) setSettingsInfo(notice,'Не удалось загрузить список версий. Повторите позже.',infoRevision);
    } finally { hysteriaVersionsLoading = false;if(window.refreshCoreUpdateBadges)window.refreshCoreUpdateBadges(); }
}

async function installHysteria(btn) {
    var selected=document.getElementById('hysteria_version_select');
    var version=selected?selected.value:'';if(!version)return;
    var orig = btn.innerText; btn.innerText = 'Установка...'; btn.disabled = true;
    try {
        var res = await (await fetch('/api/hysteria_action', {method:'POST',
            headers:{'Content-Type':'application/x-www-form-urlencoded'},
            body:'action=install&version='+encodeURIComponent(version)})).json();
        alert(res.message); await checkHysteria();
    } catch(e) { alert('Ошибка: ' + e.message); }
    finally { btn.innerText = orig; btn.disabled = false; }
}
async function uninstallHysteria(btn) {
    if (!confirm('Удалить ядро Hysteria?')) return;
    var orig = btn.innerText; btn.innerText = 'Удаление...'; btn.disabled = true;
    try {
        var res = await (await fetch('/api/hysteria_action', {method:'POST',
            headers:{'Content-Type':'application/x-www-form-urlencoded'},
            body:'action=uninstall'})).json();
        alert(res.message); await checkHysteria();
    } catch(e) {}
    finally { btn.innerText = orig; btn.disabled = false; }
}

async function autoSelectSub(subId, btn) {
    var orig = btn.innerText;
    btn.innerText = 'Подключение...';
    btn.disabled = true;
    showWaitModal('Автовыбор сервера...', 'Замер задержек и подключение');
    try {
        var fd = new URLSearchParams();
        fd.append('sub_id', subId);
        var res = await fetch('/api/auto_select', { method: 'POST', body: fd });
        var payload = {};
        try { payload = await res.json(); } catch(e) {}
        if (res.ok) {
            if (payload.pings && typeof payload.pings === 'object') {
                try {
                    /* 1. Сбрасываем бейджи у ВСЕХ карточек этой подписки в DOM:
                       старые значения не должны путать пользователя. */
                    document.querySelectorAll('.server-card.sub-group-' + subId).forEach(function(c) {
                        var b = c.querySelector('.ping-badge');
                        if (b) { b.innerText = '-- ms'; b.className = 'ping-badge'; }
                    });
                    /* 2. Удаляем старые пинги этой подписки из кэша. */
                    var cache = JSON.parse(localStorage.getItem('optiray_ping_cache') || '{}');
                    document.querySelectorAll('.server-card.sub-group-' + subId).forEach(function(c) {
                        if (c.dataset.sid) delete cache[c.dataset.sid];
                    });
                    /* 3. Добавляем только свежие пинги из ответа. */
                    Object.keys(payload.pings).forEach(function(sid) {
                        var v = payload.pings[sid];
                        if (v && v.text && v.cls) {
                            cache[sid] = { text: v.text, cls: v.cls, min: v.min || 0, max: v.max || 0 };
                        }
                    });
                    localStorage.setItem('optiray_ping_cache', JSON.stringify(cache));
                } catch(e) {}
            }
            updateWaitModal('Прогрев туннеля...');
            var probeTarget2 = _testUrlFromDom();
            for (var wAttempt = 1; wAttempt <= 12; wAttempt++) {
                if (await probeUrl(probeTarget2, 1500)) break;
                updateWaitModal('Прогрев туннеля... (' + wAttempt + '/12)');
                await new Promise(function(r) { setTimeout(r, 300); });
            }
            hideWaitModal();
            if (payload.warning) {
                /* тихо, без alert: сервер уже подключён */
                if (window.console) console.log('optiray:', payload.name, '-', payload.warning);
            }
            window.location.reload();
        } else {
            hideWaitModal();
            alert('Ошибка автовыбора: ' + (payload.error || 'не удалось'));
            btn.innerText = orig;
            btn.disabled = false;
        }
    } catch(e) {
        hideWaitModal();
        alert('Сетевая ошибка: ' + e.message);
        btn.innerText = orig;
        btn.disabled = false;
    }
}

function openSettingsModal() {
    selectSettingsTab("connection");
    document.getElementById('settings_modal').style.display = 'flex';
    window.optiraySettingsReturnFocus = document.activeElement;
    var selected = document.querySelector("#settings_modal [aria-current=page]");
    if (selected) selected.focus();
}

function closeSettingsModal() { document.getElementById('settings_modal').style.display = 'none'; if (window.optiraySettingsReturnFocus) window.optiraySettingsReturnFocus.focus(); }
function backupDownloadFilename(response, fallback) {
    var disposition = response.headers.get('Content-Disposition') || '';
    var encoded = disposition.match(/filename\*=UTF-8''([^;]+)/i);
    if (encoded) {
        try { return decodeURIComponent(encoded[1].trim()); } catch (e) {}
    }
    var plain = disposition.match(/filename="?([^"\n;]+)"?/i);
    return (plain && plain[1]) || fallback;
}

function downloadBackup() {
    var optirout = document.querySelector('input[name="backup_include_optirout"]');
    var xray = document.querySelector('input[name="backup_include_xray"]');
    window.location.href = '/api/backup?optirout=' + (optirout && optirout.checked ? '1' : '0') +
        '&xray=' + (xray && xray.checked ? '1' : '0');
}

var optirayRestoreBusy = false;
async function restoreBackupFromFile(inp) {
    if (optirayRestoreBusy || !inp.files || !inp.files[0]) return;
    var file=inp.files[0];
    optirayRestoreBusy=true;
    inp.disabled=true;
    showMaintenanceWait('restore','Восстановление OptiRay','Чтение резервной копии…');
    var elapsed=0;
    var timer=null;
    try {
        var payload=JSON.parse(await file.text());
        updateWaitModal('Сохранение настроек и применение конфигурации на роутере…');
        timer=setInterval(function(){
            elapsed++;
            updateWaitModal('Восстановление на роутере… '+elapsed+' с. Дождитесь завершения.');
        },1000);
        var response=await fetch('/api/restore', {
            method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)
        });
        var result=await response.json();
        if(!response.ok || result.ok===false)throw new Error(result.error || 'Не удалось восстановить копию');
        clearInterval(timer);timer=null;
        updateWaitModal('Восстановление завершено. Обновление страницы…');
        clearAllPingCache();
        window.location.reload();
    } catch(error) {
        hideWaitModal();
        alert('Ошибка восстановления: '+error.message);
    } finally {
        if(timer)clearInterval(timer);
        inp.disabled=false;inp.value='';optirayRestoreBusy=false;
    }
}
async function resetToDefaults() {
    var msg = 'ВНИМАНИЕ!\n\n' +
              'Это действие сбросит ВСЁ:\n' +
              '• Все подписки и серверы\n' +
              '• Избранное\n' +
              '• Активное подключение\n' +
              '• Настройки HWID, прокси, PING\n' +
              '• Правила маршрутизации Xray и OptiRout\n' +
              '• Удалит конфиг Xray\n\n' +
              'Отменить это невозможно!\n\n' +
              'Продолжить?';
    if (!confirm(msg)) return;
    if (!confirm('Последнее подтверждение. Точно сбросить ВСЁ?')) return;
    if(optirayRestoreBusy)return;
    optirayRestoreBusy=true;
    showMaintenanceWait('reset','Сброс OptiRay','Возврат настроек к исходным значениям…');
    var resetSeconds=0;
    var resetTimer=setInterval(function(){updateWaitModal('Сброс на роутере… '+(++resetSeconds)+' с. Дождитесь завершения.');},1000);
    try {
        var res = await fetch('/api/reset_defaults', { method: 'POST' });
        var result = await res.json();
        if (!res.ok || !result.ok) throw new Error(result.error || ('HTTP ' + res.status));
        clearAllPingCache(); localStorage.clear(); window.location.reload();
    } catch(e) { hideWaitModal(); alert('Ошибка сброса настроек: ' + e.message); } finally { clearInterval(resetTimer); optirayRestoreBusy=false; }
}

function scrollToTop() { window.scrollTo({ top: 0, behavior: 'smooth' }); }

/* ==== Routing: кнопка в шапке + отдельное модальное окно ==== */
function handleRoutingClick(btn) {
    if (!btn) return;
    var engine = btn.getAttribute('data-routing-engine') || 'xray';
    if (engine === 'optirout') {
        openMtModal();
        return;
    }
    var isExt = btn.getAttribute('data-routing-external') === '1';
    var url = btn.getAttribute('data-routing-url') || '';
    if (isExt && /^https?:\/\//i.test(url)) {
        try { window.open(url, '_blank', 'noopener'); } catch (e) {}
    } else {
        openRoutingModal();
    }
}

function openRoutingModal() {
    var m = document.getElementById('routing_modal');
    if (!m) return;
    m.style.display = 'flex';
    if (typeof window._optirayRoutingKick === 'function') {
        try { window._optirayRoutingKick(); } catch (e) {}
    }
}


/* ==== OptiRout: модалка редактирования config.json ==== */
var MT_STATE = { config: null, interfaces: [], _dirty: false };

/* dirty — теперь property с getter/setter.
   Любая запись `MT_STATE.dirty = ...` автоматически обновляет
   визуальное состояние кнопки «Сохранить» (dim / active). */
Object.defineProperty(MT_STATE, 'dirty', {
    get: function() { return this._dirty; },
    set: function(v) {
        if(this.config){
            try{_mtCollectFromUi(true);}catch(error){}
            var current=routingConfigSignature('optirout',this.config);
            if(!v)this._savedSignature=current;
            this._dirty=this._savedSignature==null?!!v:current!==this._savedSignature;
        }else{this._savedSignature=null;this._dirty=false;}
        if (typeof _mtSetSaveDirty === 'function') _mtSetSaveDirty(this._dirty);
    }
});

var MT_UI = { collapsed: {} };

/* Переключает визуальное состояние кнопки «Сохранить».
   isDirty=false → приглушённая (нечего сохранять)
   isDirty=true  → активная (есть изменения) */
function _mtSetSaveDirty(isDirty) {
    var modal=document.getElementById('mt_modal');if(modal&&modal.dataset.ruleEngine==='xray')return;
    var notice = document.getElementById('mt_dirty_notice');
    if (notice) notice.textContent = isDirty ? 'Есть несохранённые изменения' : '';
    var btn = document.getElementById('mt_save_btn');
    if (!btn) return;
    btn.disabled=!isDirty;btn.setAttribute('aria-disabled',String(!isDirty));
    if (isDirty) {
        btn.classList.remove('mt-save-idle');
    } else {
        btn.classList.add('mt-save-idle');
    }
}

function _mtEsc(s) {
    return String(s == null ? '' : s)
        .replace(/&/g, '&amp;').replace(/</g, '&lt;')
        .replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

/* Генерирует ID в формате OptiRout: ровно 8 hex-символов.
   Другие форматы ломают парсер OptiRout и приводят к потере групп. */
var OPTIRAY_AUTO_COLOR_ICON = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3a9 9 0 1 0 0 18h1a2 2 0 0 0 1.5-3.3 1.5 1.5 0 0 1 1.1-2.5H17a4 4 0 0 0 4-4c0-4.4-4-8.2-9-8.2Z"/><circle cx="7.5" cy="10" r="1" fill="currentColor" stroke="none"/><circle cx="10.5" cy="6.8" r="1" fill="currentColor" stroke="none"/><circle cx="15" cy="7.2" r="1" fill="currentColor" stroke="none"/><circle cx="17.5" cy="10.5" r="1" fill="currentColor" stroke="none"/></svg>';

function randomRoutingGroupColor() {
    // Pick a hue once when creating a group; its saved color stays unchanged.
    var hue = Math.random() * 6;
    var chroma = 0.60;
    var x = chroma * (1 - Math.abs(hue % 2 - 1));
    var rgb = hue < 1 ? [chroma,x,0] : hue < 2 ? [x,chroma,0] :
              hue < 3 ? [0,chroma,x] : hue < 4 ? [0,x,chroma] :
              hue < 5 ? [x,0,chroma] : [chroma,0,x];
    return '#' + rgb.map(function(channel) {
        return ('0' + Math.round((channel + 0.20) * 255).toString(16)).slice(-2);
    }).join('');
}

function _mtNewId() {
    var b = new Uint8Array(4);
    (window.crypto || window.msCrypto).getRandomValues(b);
    var s = '';
    for (var i = 0; i < b.length; i++) {
        var h = b[i].toString(16);
        s += (h.length === 1 ? '0' + h : h);
    }
    return s;
}

/* Совместимость: интерфейс может быть строкой (старый формат)
   или {name, desc} (новый). Обе формы поддерживаются. */
function _mtIfaceName(item) {
    if (typeof item === 'string') return item;
    if (item && typeof item === 'object' && item.name) return String(item.name);
    return '';
}
function _mtIfaceDesc(item) {
    if (item && typeof item === 'object' && item.desc) return String(item.desc);
    return '';
}
function _mtIfaceLabel(item) {
    var n = _mtIfaceName(item);
    var d = _mtIfaceDesc(item);
    return d ? (n + ' \u2014 ' + d) : n;
}

function _mtSetStatus(text, kind) {
    var el = document.getElementById('mt_status_bar');
    if (!el) return;
    el.textContent = text || '';
    el.className = 'mt-status' + (kind ? ' ' + kind : '');
}

/* Показать/скрыть кнопку «Бэкапы» в шапке mt_modal.
   Управляется значением window.OPTIRAY_OPTIROUT_SAVE_BACKUPS,
   которое прошивается из settings.optirout_save_backups при рендере страницы. */
function _mtToggleBackupBtn() {
    var btn = document.querySelector('#mt_modal .rt-import-export-btn.mt-io-backup');
    var enabled = (typeof window.OPTIRAY_OPTIROUT_SAVE_BACKUPS === 'undefined')
        ? false
        : !!window.OPTIRAY_OPTIROUT_SAVE_BACKUPS;
    if (btn) {
        btn.dataset.backupVisible = String(enabled);
        if (enabled) btn.style.removeProperty('display');
        else btn.style.setProperty('display', 'none', 'important');
    }
    var toggle = document.getElementById('mt_backup_enabled');
    if (toggle) toggle.checked = enabled;
}

async function mtSetBackupEnabled(toggle) {
    var previous = !!window.OPTIRAY_OPTIROUT_SAVE_BACKUPS;
    toggle.disabled = true;
    try {
        var response = await fetch('/api/optirout_backup_settings', {
            method:'POST', headers:{'Content-Type':'application/json'},
            body:JSON.stringify({enabled:toggle.checked})
        });
        var result = await response.json();
        if (!response.ok || !result.ok) throw new Error(result.error || 'Не удалось сохранить настройку');
        window.OPTIRAY_OPTIROUT_SAVE_BACKUPS = result.enabled;
        _mtToggleBackupBtn();
    } catch (error) {
        toggle.checked = previous;
        alert('Не удалось сохранить настройку бэкапов: ' + error.message);
    } finally {
        toggle.disabled = false;
    }
}

async function openMtModal() {
    document.querySelectorAll('#mt_modal .mt-io-backup').forEach(function(btn){btn.dataset.backupVisible='false';});
    var m = document.getElementById('mt_modal');
    if (!m) return;
    m.style.display = 'flex';
    _mtSetStatus('Загрузка конфига...', '');
    MT_STATE.config = null;
    MT_STATE.dirty = false;
    try {
        var r1 = await fetch('/api/optirout_config', {cache:'no-store'});
        var d1 = await r1.json();
        if (!d1.ok) {
            _mtSetStatus('Ошибка: ' + (d1.error || 'unknown'), 'err');
            return;
        }
        MT_STATE.config = d1.config;
        window.OPTIRAY_OPTIROUT_SAVE_BACKUPS = d1.save_backups === true;
        _mtToggleBackupBtn();
 var power=document.getElementById("mt_enabled");if(power)power.checked=MT_STATE.config.enabled!==false;
        var r2 = await fetch('/api/optirout_interfaces');
        var d2 = await r2.json();
        MT_STATE.interfaces = (d2 && d2.interfaces) || [];

        /* При каждом открытии модалки — все группы свёрнуты.
           Состояние хранится только в UI (MT_UI.collapsed) и не пишется в YAML. */
        MT_UI.collapsed = {};
        var _grps = MT_STATE.config.groups || [];
        for (var _gi = 0; _gi < _grps.length; _gi++) {
            if (_grps[_gi] && _grps[_gi].id) MT_UI.collapsed[_grps[_gi].id] = true;
        }

        /* Глобальный интерфейс: из localStorage или первый доступный */
        try {
            var _savedIface = localStorage.getItem('optiray_mt_global_iface') || '';
            if (_savedIface) MT_UI.globalIface = _savedIface;
        } catch(e) {}
        if (!MT_UI.globalIface) {
            for (var _ii = 0; _ii < MT_STATE.interfaces.length; _ii++) {
                var _n = _mtIfaceName(MT_STATE.interfaces[_ii]);
                if (_n) { MT_UI.globalIface = _n; break; }
            }
        }

        mtRenderGlobal();
        mtRenderGroups();
        MT_STATE.dirty = false;   /* триггерит setter → кнопка «Сохранить» dim */
        _mtSetStatus('Конфиг загружен. ' + ruCount((MT_STATE.config.groups || []).length, 'группа', 'группы', 'групп'), 'ok');

        /* Скрыть кнопку «Бэкапы» если сохранение бэкапов отключено */
        _mtToggleBackupBtn();
    } catch (e) {
        _mtSetStatus('Сеть: ' + e.message, 'err');
    }
}

function closeMtModal() {
    if (MT_STATE.dirty && !confirm('Закрыть без сохранения изменений?')) return;
    var m = document.getElementById('mt_modal');
    if (m) m.style.display = 'none';
    MT_STATE.config = null;
    MT_STATE.interfaces = [];
    MT_STATE.dirty = false;
}

function mtRenderGlobal() {
    var cfg = MT_STATE.config || {};
    var app = cfg.app || {};
    var link = app.link || [];
    if (typeof link === 'string') link = [link];

    var cont = document.getElementById('mt_link_container');
    if (cont) {
        var html = '';
        for (var i = 0; i < MT_STATE.interfaces.length; i++) {
            var item = MT_STATE.interfaces[i];
            var iface = _mtIfaceName(item);
            if (!iface) continue;
            var checked = link.indexOf(iface) >= 0;
            html += '<label style="display:inline-flex;align-items:center;gap:4px;font-size:var(--text-size);padding:3px 8px;border:1px solid var(--line);border-radius:999px;cursor:pointer;">'
                + '<input type="checkbox" class="mt-link-chk" value="' + _mtEsc(iface) + '"' + (checked ? ' checked' : '') + '>'
                + _mtEsc(_mtIfaceLabel(item)) + '</label>';
        }
        cont.innerHTML = html;
    }
    var sa = document.getElementById('mt_show_all');
    if (sa) sa.checked = !!app.showAllInterfaces;
    var dp = document.getElementById('mt_dns_port');
    if (dp) dp.value = (app.dnsProxy && app.dnsProxy.host && app.dnsProxy.host.port) || 3553;
    var wp = document.getElementById('mt_web_port');
    if (wp) wp.value = (app.httpWeb && app.httpWeb.host && app.httpWeb.host.port) || 8080;
    var ll = document.getElementById('mt_loglevel');
    if (ll) ll.value = app.logLevel || 'info';
}

function _mtIfaceOptions(current) {
    var html = '';
    var seen = {blackhole:1};
    for (var i = 0; i < MT_STATE.interfaces.length; i++) {
        var item = MT_STATE.interfaces[i];
        var n = _mtIfaceName(item);
        if (!n || (n === 'br0' && !document.getElementById('settings_show_br0')?.checked)) continue;
        seen[n] = 1;
        var d = _mtIfaceDesc(item) || '';
        var label = d ? (n + ' \u2014 ' + d) : n;
        html += '<option value="' + _mtEsc(n) + '" data-name="' + _mtEsc(n) + '" data-desc="' + _mtEsc(d) + '"' + (n === current ? ' selected' : '') + '>' + _mtEsc(label) + '</option>';
    }
    if (current && (current !== 'br0' || document.getElementById('settings_show_br0')?.checked) && !seen[current]) {
        html = '<option value="' + _mtEsc(current) + '" data-name="' + _mtEsc(current) + '" data-desc="" selected>' + _mtEsc(current) + '</option>' + html;
    }
    return html;
}

function _mtIfacePickerHtml(current, gid) {
    var curName = current || 'blackhole';
    var curDesc = '';
    for (var i = 0; i < MT_STATE.interfaces.length; i++) {
        var item = MT_STATE.interfaces[i];
        if (_mtIfaceName(item) === curName) {
            curDesc = _mtIfaceDesc(item) || '';
            break;
        }
    }
    var displayName = curName === 'blackhole' ? 'Блокировать' : curName;
    var descStyle = curDesc ? '' : 'display:none;';
    return '<div class="mt-iface-picker" data-gid="' + _mtEsc(gid) + '">'
        + '<select class="mt-group-iface" title="Интерфейс" tabindex="-1" aria-hidden="true">'
        + '<option value="blackhole" data-name="Блокировать" data-desc=""' + (curName === 'blackhole' ? ' selected' : '') + '>Блокировать</option>'
        + _mtIfaceOptions(curName)
        + '</select>'
        + '<button type="button" class="mt-iface-display" aria-label="Выбрать интерфейс группы" aria-haspopup="menu" aria-expanded="false">'
        + '<div class="mt-iface-name">' + _mtEsc(displayName) + '</div>'
        + '<div class="mt-iface-desc" style="' + descStyle + '">' + _mtEsc(curDesc) + '</div>'
        + '</button>'
        + '</div>';
}

var OPTIRAY_ROUTING_ICONS = {
    drag: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="9" cy="5" r="1"/><circle cx="15" cy="5" r="1"/><circle cx="9" cy="12" r="1"/><circle cx="15" cy="12" r="1"/><circle cx="9" cy="19" r="1"/><circle cx="15" cy="19" r="1"/></svg>',
    delete: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 6h18M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M5 6l1 14a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1l1-14M10 10v7m4-7v7"/></svg>',
    bulk: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="5" y="5" width="14" height="16" rx="2"/><rect x="9" y="3" width="6" height="4" rx="1"/><path d="M9 12h6m-6 4h4"/></svg>',
    copy: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="8" y="8" width="13" height="13" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/></svg>',
    toggle: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m9 5 7 7-7 7"/></svg>'
};

function _mtRuleHtml(gid, r) {
    var off = (r.enable === false) ? ' mt-rule-off' : '';
    var typeOpts = ['namespace', 'wildcard', 'subnet'].map(function(t) {
        return '<option value="' + t + '"' + (t === r.type ? ' selected' : '') + '>' + t + '</option>';
    }).join('');
    return '<div class="mt-rule' + off + '" data-cat="mt" data-gid="' + _mtEsc(gid) + '" data-rid="' + _mtEsc(r.id) + '">'
        + '<span class="mt-rule-drag" title="Перетащить"><svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="9" cy="5" r="1"/><circle cx="15" cy="5" r="1"/><circle cx="9" cy="12" r="1"/><circle cx="15" cy="12" r="1"/><circle cx="9" cy="19" r="1"/><circle cx="15" cy="19" r="1"/></svg></span>'
        + '<label class="rt-check" title="Включить"><input type="checkbox" class="mt-rule-enable"' + (r.enable !== false ? ' checked' : '') + '></label>'
        + '<input type="text" class="mt-rule-input" value="' + _mtEsc(r.rule || '') + '" placeholder="example.com / 1.2.3.0/24">'
        + '<div class="mt-rule-controls" role="group" aria-label="Тип и удаление правила">'
        + '<select class="mt-rule-type">' + typeOpts + '</select>'
        + '<button type="button" class="rt-icon-btn rt-rule-del" title="Удалить">' + OPTIRAY_ROUTING_ICONS.delete + '</button>'
        + '</div>'
        + '</div>';
}

function _mtGroupHtml(g) {
    var isCollapsed = !!(MT_UI && MT_UI.collapsed && MT_UI.collapsed[g.id]);
    var collapsed = isCollapsed ? ' collapsed' : '';
    var off = (g.enable === false) ? ' mt-group-off' : '';
    var rulesHtml = '';
    var rs = g.rules || [];
    if (!rs.length) rs = [{id: _mtNewId(), type: 'namespace', rule: '', enable: true}];
    for (var i = 0; i < rs.length; i++) rulesHtml += _mtRuleHtml(g.id, rs[i]);
    return '<div class="mt-group-card' + collapsed + off + '" style="--rule-group-color:' + (/^#[0-9a-f]{6}$/i.test(g.color || '') ? g.color : '#888888') + '" data-gid="' + _mtEsc(g.id) + '">'
        + '<div class="mt-group-head">'
        +   '<span class="mt-group-drag" title="Перетащить"><svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="9" cy="5" r="1"/><circle cx="15" cy="5" r="1"/><circle cx="9" cy="12" r="1"/><circle cx="15" cy="12" r="1"/><circle cx="9" cy="19" r="1"/><circle cx="15" cy="19" r="1"/></svg></span>'
        +   '<label class="rt-check" title="Включить группу"><input type="checkbox" class="mt-group-enable"' + (g.enable !== false ? ' checked' : '') + '></label>'
        +   '<span aria-hidden="true"></span><input type="color" class="mt-group-color" value="' + _mtEsc(g.color || '#888888') + '" title="Цвет">'
        +   '<input type="text" class="mt-group-name" value="' + _mtEsc(g.name || '') + '" placeholder="Имя группы">'
        +   '<div class="mt-group-controls" role="group" aria-label="Интерфейс и действия группы">'
        +   _mtIfacePickerHtml(g.interface || '', g.id)
        +   '<div class="mt-group-actions"><button type="button" class="rt-icon-btn mt-add-rule" title="Добавить правило" aria-label="Добавить правило"><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14"/></svg></button>'
        +   '<button type="button" class="rt-icon-btn mt-group-bulk" title="Добавить правила списком">' + OPTIRAY_ROUTING_ICONS.bulk + '</button>'
        +   '<button type="button" class="rt-icon-btn mt-group-copy" title="Копировать группу в буфер">' + OPTIRAY_ROUTING_ICONS.copy + '</button>'
        +   '<button type="button" class="rt-icon-btn rt-group-del" title="Удалить группу">' + OPTIRAY_ROUTING_ICONS.delete + '</button>'
        +   '</div>'
        +   '</div>'
        +   '<button type="button" class="rt-icon-btn mt-group-toggle" title="Свернуть/Развернуть">'
        +       OPTIRAY_ROUTING_ICONS.toggle
        +   '</button>'
        + '</div>'
        + '<div class="mt-group-body">'
        +   '<div class="mt-rules">' + rulesHtml + '</div>'
        
        + '</div>'
        + '</div>';
}

function mtRenderGroups() {
 mtUpdatePowerUi();
    var cont = document.getElementById('mt_groups_container');
    if (!cont) return;
    var cfg = MT_STATE.config || {};
    var groups = cfg.groups || [];
    if (!groups.length) {
        cont.innerHTML = '<div style="text-align:center;color:var(--text-2);padding:20px;font-size:var(--text-size);">Групп нет. Нажмите «+ Добавить группу».</div>';
        return;
    }
    var html = '';
    var failed = 0;
    for (var i = 0; i < groups.length; i++) {
        try {
            html += _mtGroupHtml(groups[i]);
        } catch (e) {
            failed++;
            if (window.console && console.error) {
                console.error('mtRenderGroups: group failed', i, groups[i], e);
            }
            html += '<div class="mt-group-card"><div class="mt-group-head" style="padding:8px;">'
                + '<span style="color:var(--btn-danger-text);font-size:var(--text-size);">⚠ Ошибка рендера группы #' + i + ': '
                + _mtEsc(e.message || e) + '</span></div></div>';
        }
    }
    cont.innerHTML = html;
    mtFilterGroups();
    if (failed > 0) {
        _mtSetStatus('Внимание: ' + failed + ' групп(ы) не отрендерились. Не сохраняйте до исправления!', 'err');
    }
}

function _mtCollectFromUi(silent) {
    var cfg = MT_STATE.config;
    if (!cfg) return true;

    // global
    var app = cfg.app || (cfg.app = {});
    app.showAllInterfaces = !!(document.getElementById('mt_show_all') || {}).checked;
    /*
     * FIX: не удалять скрытые/служебные link-интерфейсы.
     * Например br0 может отсутствовать в списке UI, но уже присутствовать
     * в app.link. Старый код превращал такой link в [] при сохранении.
     */
    var existingLinks = Array.isArray(app.link)
        ? app.link.slice()
        : (app.link ? [app.link] : []);

    var links = [];
    var visibleLinks = {};

    document.querySelectorAll('.mt-link-chk').forEach(function(c) {
        visibleLinks[c.value] = true;
        if (c.checked) links.push(c.value);
    });

    /* Сохраняем интерфейсы, которых нет в текущем UI */
    existingLinks.forEach(function(v) {
        if (v && !visibleLinks[v] && links.indexOf(v) < 0) {
            links.push(v);
        }
    });

    app.link = links;
    var dns = app.dnsProxy || (app.dnsProxy = {});
    var dnsHost = dns.host || (dns.host = {});
    dnsHost.port = parseInt((document.getElementById('mt_dns_port') || {}).value, 10) || 3553;
    var web = app.httpWeb || (app.httpWeb = {});
    var webHost = web.host || (web.host = {});
    webHost.port = parseInt((document.getElementById('mt_web_port') || {}).value, 10) || 8080;
    app.logLevel = (document.getElementById('mt_loglevel') || {}).value || 'info';

    // groups — защита от потери
    var cont = document.getElementById('mt_groups_container');
    if (!cont) return true;

    var cardsInDom = cont.querySelectorAll('.mt-group-card').length;
    var groupsInCfg = (cfg.groups || []).length;

    // Считаем "сломанные" карточки — те, что отрендерились с ошибкой
    var brokenCards = 0;
    cont.querySelectorAll('.mt-group-card').forEach(function(c) {
        if (!c.getAttribute('data-gid')) brokenCards++;
    });

    if (brokenCards > 0) {
        if (!silent) {
            _mtSetStatus('⚠ Есть ' + brokenCards + ' сломанных групп — сохранение отменено', 'err');
        }
        return false;
    }
    if (cardsInDom !== groupsInCfg) {
        if (!silent) {
            _mtSetStatus('⚠ Расхождение: DOM ' + cardsInDom + ' vs данные ' + groupsInCfg + ' — сохранение отменено', 'err');
        }
        return false;
    }

    var newGroups = [];
    cont.querySelectorAll('.mt-group-card').forEach(function(card) {
        var gid = card.getAttribute('data-gid');
        var g = null;
        for (var i = 0; i < (cfg.groups || []).length; i++) {
            if (cfg.groups[i].id === gid) { g = cfg.groups[i]; break; }
        }
        if (!g) g = { id: gid, name: '', color: '#888888', interface: '', enable: true, rules: [] };
        g.name = (card.querySelector('.mt-group-name') || {}).value || '';
        g.color = (card.querySelector('.mt-group-color') || {}).value || '#888888';
        g.interface = (card.querySelector('.mt-group-iface') || {}).value || '';
        g.enable = !!(card.querySelector('.mt-group-enable') || {}).checked;
        // collapsed НЕ пишем в config (иначе OptiRout не загрузит файл) —
        // держим только в UI-состоянии MT_UI.collapsed
        if (MT_UI && MT_UI.collapsed) {
            MT_UI.collapsed[gid] = card.classList.contains('collapsed');
        }
        var newRules = [];
        card.querySelectorAll('.mt-rule').forEach(function(rEl) {
            var rid = rEl.getAttribute('data-rid');
            var r = { id: rid, name: '', type: 'namespace', rule: '', enable: true };
            for (var j = 0; j < (g.rules || []).length; j++) {
                if (g.rules[j].id === rid) {
                    r.name = g.rules[j].name || '';
                    break;
                }
            }
            r.type = (rEl.querySelector('.mt-rule-type') || {}).value || 'namespace';
            r.rule = (rEl.querySelector('.mt-rule-input') || {}).value || '';
            r.enable = !!(rEl.querySelector('.mt-rule-enable') || {}).checked;
            if (r.rule.trim()) newRules.push(r);
        });
        g.rules = newRules.length ? newRules : [{ id: _mtNewId(), name: '', type: 'namespace', rule: '', enable: true }];
        newGroups.push(g);
    });
    cfg.groups = newGroups;
    return true;
}

window.mtAddGroup = function() {
    _mtCollectFromUi();
    var cfg = MT_STATE.config;
    if (!cfg) return;
    cfg.groups = cfg.groups || [];
    cfg.groups.unshift({
        id: _mtNewId(),
        name: 'Новая группа',
        color: randomRoutingGroupColor(),
        interface: (_mtIfaceName(MT_STATE.interfaces[0]) || 't2s0'),
        enable: true,
        collapsed: false,
        rules: [{ id: _mtNewId(), name: '', type: 'namespace', rule: '', enable: true }]
    });
    mtRenderGroups();
    MT_STATE.dirty = true;
};

window.mtExport = function() {
    _mtCollectFromUi();
    var cfg = MT_STATE.config || {};
    fetch('/api/optirout_export', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({config: cfg})
    }).then(function(r) {
        if (!r.ok) { alert('Ошибка экспорта: HTTP ' + r.status); return; }
        return r.blob().then(function(blob) {
            var fname = backupDownloadFilename(r, 'optirout_routing.xlsx');
            var url = URL.createObjectURL(blob);
            var a = document.createElement('a');
            a.href = url; a.download = fname;
            document.body.appendChild(a); a.click(); document.body.removeChild(a);
            setTimeout(function() { try { URL.revokeObjectURL(url); } catch(e) {} }, 3000);
        });
    }).catch(function(e) { alert('Сеть: ' + e.message); });
};

window.mtImportFile = function(inp) {
    if (!inp || !inp.files || !inp.files[0]) return;
    var file = inp.files[0];
    if (!confirm('Импортировать правила из "' + file.name + '"?\n\nВсе текущие группы будут заменены.')) {
        inp.value = ''; return;
    }
    var r = new FileReader();
    r.onload = function() {
        var s = String(r.result || '');
        var c = s.indexOf(',');
        var b64 = c >= 0 ? s.slice(c + 1) : s;
        fetch('/api/optirout_import', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({file_b64: b64})
        }).then(function(resp) {
            return resp.json().then(function(d) {
                inp.value = '';
                if (!resp.ok || !d.ok) {
                    alert('Ошибка импорта: ' + ((d && d.error) || ('HTTP ' + resp.status)));
                    return;
                }
                MT_STATE.config.groups = d.groups || [];
                mtRenderGroups();
                _mtSetStatus('Импорт: ' + ruCount(MT_STATE.config.groups.length, 'группа', 'группы', 'групп'), 'ok');
                MT_STATE.dirty = true;
            });
        }).catch(function(e) { alert('Сеть: ' + e.message); inp.value = ''; });
    };
    r.readAsDataURL(file);
};

window.saveMtConfig = function() {
    if(!MT_STATE.dirty)return;
    if (!_mtCollectFromUi(false)) {
        // _mtCollectFromUi вернул false — есть расхождение, показали ошибку, не сохраняем
        return;
    }
    var cfg = MT_STATE.config;
    if (!cfg) return;
    var btn = document.getElementById('mt_save_btn');
    var orig = btn ? btn.textContent : '';
    if (btn) { btn.disabled = true; btn.textContent = 'Сохранение...'; }
    _mtSetStatus('Сохранение и перезапуск OptiRout...', '');
    fetch('/api/optirout_config', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({config: cfg, restart: true})
    }).then(function(r) {
        return r.json().then(function(d) {
            if (btn) { btn.disabled = false; btn.textContent = orig; }
            if (!r.ok || !d.ok) {
                _mtSetStatus('Ошибка: ' + ((d && d.error) || ('HTTP ' + r.status)), 'err');
                return;
            }
            _mtSetStatus(d.restarted ? 'Сохранено. OptiRout запущен.' : 'Сохранено. OptiRout не запущен.', 'ok');
            MT_STATE.dirty = false;
        });
    }).catch(function(e) {
        if (btn) { btn.disabled = false; btn.textContent = orig; }
        _mtSetStatus('Сеть: ' + e.message, 'err');
    });
};

window.mtOpenGlobalIface = function(btn) {
    if (!MT_STATE.interfaces || !MT_STATE.interfaces.length) {
        alert('Нет доступных интерфейсов');
        return;
    }
    if (!btn) {
        btn = document.querySelector('#mt_modal .mt-global-iface-btn');
    }
    if (!btn) return;

    var dd = document.getElementById('mt_global_iface_dd');
    if (!dd) return;

    /* Если уже открыт — закрываем (toggle) */
    if (dd.style.display === 'block') {
        _mtCloseGlobalIface();
        return;
    }

    /* Строим список */
    var html = '';
    for (var i = 0; i < MT_STATE.interfaces.length; i++) {
        var item = MT_STATE.interfaces[i];
        var n = _mtIfaceName(item);
        if (!n || (n === 'br0' && !document.getElementById('settings_show_br0')?.checked)) continue;
        var d = _mtIfaceDesc(item) || '';
        var isCur = (n === MT_UI.globalIface);
        html += '<button type="button" role="menuitemradio" aria-checked="' + String(isCur) + '" class="mt-global-iface-dd-item' + (isCur ? ' is-current' : '') + '"'
             +  ' data-iface="' + _mtEsc(n) + '">'
             +  '<div class="mt-global-iface-dd-name">' + _mtEsc(n) + '</div>'
             +  (d ? '<div class="mt-global-iface-dd-desc">' + _mtEsc(d) + '</div>' : '')
             +  '</button>';
    }
    if (!html) {
        alert('Нет доступных интерфейсов');
        return;
    }
    dd.innerHTML = html;
    dd.classList.add('is-attached');
    dd.setAttribute('role','menu');
    dd.setAttribute('aria-label','Интерфейс для всех групп');
    btn.setAttribute('aria-haspopup','menu');
    btn.setAttribute('aria-expanded','true');

    /* Позиционируем под кнопкой */
    var rect = btn.getBoundingClientRect();
    dd.style.display = 'block';
    var buttonTop = rect.top - (btn.offsetHeight - rect.height) / 2;
    var buttonBottom = buttonTop + btn.offsetHeight;
    var ddH = dd.offsetHeight;
    var ddW = dd.offsetWidth;
    var spaceBelow = window.innerHeight - buttonBottom;
    var top;
    if (spaceBelow >= ddH + 12) {
        top = buttonBottom;
        dd.dataset.side = 'below';
    } else {
        top = buttonTop - ddH;
        dd.dataset.side = 'above';
        if (top < 8) top = 8;
    }
    var left = rect.left - (btn.offsetWidth - rect.width) / 2;
    if (left + ddW > window.innerWidth - 8) {
        left = window.innerWidth - ddW - 8;
    }
    if (left < 8) left = 8;
    dd.style.top = top + 'px';
    dd.style.left = left + 'px';

    /* Обработчик клика вне */
    setTimeout(function() {
        document.addEventListener('click', _mtGlobalIfaceOutsideClick, true);
        window.addEventListener('scroll', _mtCloseGlobalIface, true);
        window.addEventListener('resize', _mtCloseGlobalIface, true);
    }, 0);
};

function _mtGlobalIfaceOutsideClick(e) {
    var dd = document.getElementById('mt_global_iface_dd');
    if (!dd || dd.style.display !== 'block') return;
    if (dd.contains(e.target)) return;
    if (e.target.closest && e.target.closest('.mt-global-iface-btn')) return;
    _mtCloseGlobalIface();
}

function _mtCloseGlobalIface() {
    var dd = document.getElementById('mt_global_iface_dd');
    if (dd) dd.style.display = 'none';
    var btn = document.querySelector('#mt_modal .mt-global-iface-btn');
    if (btn) btn.setAttribute('aria-expanded','false');
    document.removeEventListener('click', _mtGlobalIfaceOutsideClick, true);
    window.removeEventListener('scroll', _mtCloseGlobalIface, true);
    window.removeEventListener('resize', _mtCloseGlobalIface, true);
}

window.mtApplyGlobalIface = function(chosen) {
    if (!chosen) return;
    MT_UI.globalIface = chosen;
    try { localStorage.setItem('optiray_mt_global_iface', chosen); } catch(e) {}
    var cfg = MT_STATE.config;
    if (!cfg) return;
    var groups = cfg.groups || [];
    for (var i = 0; i < groups.length; i++) {
        groups[i].interface = chosen;
    }
    mtRenderGroups();
    MT_STATE.dirty = true;
    if (typeof _mtSetStatus === 'function') {
        _mtSetStatus('Интерфейс \u00ab' + chosen + '\u00bb установлен для ' + groups.length + ' групп', 'ok');
    }
};

/* Клик по пункту dropdown */
document.addEventListener('click', function(e) {
    var t = e.target;
    if (!t || !t.closest) return;
    var item = t.closest('.mt-global-iface-dd-item');
    if (!item) return;
    var iface = item.getAttribute('data-iface');
    if (!iface) return;
    e.stopPropagation();
    e.preventDefault();
    window.mtApplyGlobalIface(iface);
    _mtCloseGlobalIface();
}, true);

/* Esc — закрыть dropdown */
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        _mtCloseGlobalIface();
    }
}, true);

window.mtReloadConfig = async function(btn) {
    var orig = btn ? btn.innerHTML : '';
    if (btn) { btn.disabled = true; }
    _mtSetStatus('Перезагрузка конфига...', '');
    try {
        var r1 = await fetch('/api/optirout_config', {cache:'no-store'});
        var d1 = await r1.json();
        if (!d1.ok) {
            _mtSetStatus('Ошибка: ' + (d1.error || 'unknown'), 'err');
            return;
        }
        MT_STATE.config = d1.config;
        window.OPTIRAY_OPTIROUT_SAVE_BACKUPS = d1.save_backups === true;
        _mtToggleBackupBtn();
 var power=document.getElementById("mt_enabled");if(power)power.checked=MT_STATE.config.enabled!==false;

        var r2 = await fetch('/api/optirout_interfaces');
        var d2 = await r2.json();
        MT_STATE.interfaces = (d2 && d2.interfaces) || [];

        /* Все группы заново сворачиваем */
        MT_UI.collapsed = {};
        var _grps = MT_STATE.config.groups || [];
        for (var _gi = 0; _gi < _grps.length; _gi++) {
            if (_grps[_gi] && _grps[_gi].id) MT_UI.collapsed[_grps[_gi].id] = true;
        }

        mtRenderGlobal();
        mtRenderGroups();
        MT_STATE.dirty = false;
        _mtSetStatus('Перезагружено. ' + ruCount((MT_STATE.config.groups || []).length, 'группа', 'группы', 'групп'), 'ok');
    } catch (e) {
        _mtSetStatus('Сеть: ' + e.message, 'err');
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = orig;
        }
    }
};

window.mtResetRules = async function(btn) {
    var msg = 'Сбросить ВСЕ правила и группы OptiRout?\n\n' +
              'Что произойдёт:\n' +
              '  1. Все группы и правила будут удалены\n' +
              '  2. config.json будет восстановлен по умолчанию\n' +
              '  3. OptiRout будет перезапущен\n\n' +
              (window.OPTIRAY_OPTIROUT_SAVE_BACKUPS ? 'Текущая конфигурация сохранится в бэкап.\n' : '') +
              'Отменить это действие нельзя.\n\n' +
              'Продолжить?';
    if (!confirm(msg)) return;

    var orig = btn ? btn.innerHTML : '';
    if (btn) { btn.disabled = true; }
    _mtSetStatus('Сброс правил и восстановление config.json...', '');

    try {
        var res = await fetch('/api/optirout_reset', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({})
        });
        var d = {};
        try { d = await res.json(); } catch(e) {}
        if (!res.ok || !d.ok) {
            _mtSetStatus('Ошибка сброса: ' + ((d && d.error) || ('HTTP ' + res.status)), 'err');
            return;
        }
        if (d.restarted) {
            _mtSetStatus('Сброс выполнен. OptiRout перезапущен', 'ok');
        } else {
            _mtSetStatus('Сброс выполнен. Файл восстановлен по умолчанию', 'ok');
        }
        /* Перечитываем конфиг и перерисовываем список групп */
        try {
            var r1 = await fetch('/api/optirout_config', {cache:'no-store'});
            var d1 = await r1.json();
            if (d1 && d1.ok) {
                MT_STATE.config = d1.config;
        window.OPTIRAY_OPTIROUT_SAVE_BACKUPS = d1.save_backups === true;
        _mtToggleBackupBtn();
 var power=document.getElementById("mt_enabled");if(power)power.checked=MT_STATE.config.enabled!==false;
                MT_UI.collapsed = {};
                mtRenderGroups();
                MT_STATE.dirty = false;
            }
        } catch(e) {}
    } catch(e) {
        _mtSetStatus('Сеть: ' + e.message, 'err');
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = orig;
        }
    }
};

window.mtOpenBackups = async function() {
    try {
        var r = await fetch('/api/optirout_backups');
        var d = await r.json();
        if (!d.ok) { alert('Ошибка: ' + (d.error || 'unknown')); return; }
        if (!d.backups || !d.backups.length) { alert('Бэкапов нет.'); return; }
        var lines = d.backups.map(function(b, i) {
            return (i+1) + '. ' + b.name + ' (' + b.size + ' B)';
        });
        var sel = prompt('Выберите номер бэкапа для восстановления:\n\n' + lines.join('\n'), '1');
        if (!sel) return;
        var idx = parseInt(sel, 10) - 1;
        if (idx < 0 || idx >= d.backups.length) { alert('Неверный номер.'); return; }
        var name = d.backups[idx].name;
        if (!confirm('Восстановить конфиг из "' + name + '"?\n\nТекущий конфиг тоже будет забэкаплен.')) return;
        var r2 = await fetch('/api/optirout_restore_backup', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({name: name})
        });
        var d2 = await r2.json();
        if (!d2.ok) { alert('Ошибка: ' + (d2.error || 'unknown')); return; }
        alert('OK: восстановлено из ' + name);
        location.reload();
    } catch(e) { alert('Сеть: ' + e.message); }
};

/* ==== OptiRout: drag&drop групп и правил ==== */
(function() {
    'use strict';
    if (window.__MT_DND_INSTALLED) return;
    window.__MT_DND_INSTALLED = true;

    /* ---------- Группы ---------- */
    var dragCard = null;

    function clearGroupHints() {
        var els = document.querySelectorAll(
            '#mt_groups_container .mt-group-card.drag-over-top, ' +
            '#mt_groups_container .mt-group-card.drag-over-bottom'
        );
        for (var i = 0; i < els.length; i++) {
            els[i].classList.remove('drag-over-top');
            els[i].classList.remove('drag-over-bottom');
        }
    }

    function markDirty() {
        try { MT_STATE.dirty = true; } catch(e) {}
        try { _mtCollectFromUi(true); } catch(e) {}
    }

    /* draggable=true ставим только при mousedown на ручке группы */
    document.addEventListener('mousedown', function(e) {
        var h = e.target.closest && e.target.closest('#mt_modal .mt-group-drag');
        if (!h) return;
        var card = h.closest('#mt_groups_container .mt-group-card');
        if (!card) return;
        card.setAttribute('draggable', 'true');
    }, true);

    document.addEventListener('dragstart', function(e) {
        if (dragRule) return;   // правило уже тащат — не перебиваем
        var card = e.target.closest && e.target.closest('#mt_groups_container .mt-group-card');
        if (!card || card.getAttribute('draggable') !== 'true') return;
        dragCard = card;
        card.classList.add('dragging');
        try {
            e.dataTransfer.effectAllowed = 'move';
            e.dataTransfer.setData('text/plain', 'mt-group');
        } catch(_) {}
    }, true);

    document.addEventListener('dragover', function(e) {
        if (!dragCard) return;
        var target = e.target.closest && e.target.closest('#mt_groups_container .mt-group-card');
        if (!target || target === dragCard) return;
        if (target.parentNode !== dragCard.parentNode) return;
        e.preventDefault();
        if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
        clearGroupHints();
        var rect = target.getBoundingClientRect();
        var after = e.clientY > rect.top + rect.height / 2;
        target.classList.add(after ? 'drag-over-bottom' : 'drag-over-top');
        var parent = target.parentNode;
        parent.insertBefore(dragCard, after ? target.nextSibling : target);
    });

    document.addEventListener('drop', function(e) {
        if (!dragCard) return;
        e.preventDefault();
        e.stopPropagation();
    }, true);

    function finishGroup() {
        clearGroupHints();
        if (dragCard) {
            dragCard.classList.remove('dragging');
            dragCard.removeAttribute('draggable');
        }
        dragCard = null;
        markDirty();
    }

    document.addEventListener('dragend', function(e) {
        if (!dragCard) return;
        finishGroup();
    });

    document.addEventListener('mouseup', function() {
        var cards = document.querySelectorAll('#mt_groups_container .mt-group-card[draggable="true"]:not(.dragging)');
        for (var i = 0; i < cards.length; i++) cards[i].removeAttribute('draggable');
    }, true);

    /* ---------- Правила (внутри одной группы) ---------- */
    var dragRule = null;

    function clearRuleHints() {
        var els = document.querySelectorAll(
            '#mt_modal .mt-rule.drag-over-top, ' +
            '#mt_modal .mt-rule.drag-over-bottom'
        );
        for (var i = 0; i < els.length; i++) {
            els[i].classList.remove('drag-over-top');
            els[i].classList.remove('drag-over-bottom');
        }
    }

    document.addEventListener('mousedown', function(e) {
        var h = e.target.closest && e.target.closest('#mt_modal .mt-rule-drag');
        if (!h) return;
        var rule = h.closest('.mt-rule');
        if (!rule) return;
        rule.setAttribute('draggable', 'true');
    }, true);

    document.addEventListener('dragstart', function(e) {
        if (dragCard) return;   // группу уже тащат — не перебиваем
        var rule = e.target.closest && e.target.closest('#mt_modal .mt-rule');
        if (!rule || rule.getAttribute('draggable') !== 'true') return;
        dragRule = rule;
        rule.classList.add('dragging');
        try {
            e.dataTransfer.effectAllowed = 'move';
            e.dataTransfer.setData('text/plain', 'mt-rule');
        } catch(_) {}
        e.stopPropagation();
    }, true);

    document.addEventListener('dragover', function(e) {
        if (!dragRule) return;
        var targetRule = e.target.closest && e.target.closest('#mt_modal .mt-rule');
        if (!targetRule || targetRule === dragRule) return;
        var parent = targetRule.parentNode;
        if (!parent || !parent.classList.contains('mt-rules')) return;
        if (parent !== dragRule.parentNode) return;   // только внутри своей группы
        e.preventDefault();
        if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
        clearRuleHints();
        var rect = targetRule.getBoundingClientRect();
        var after = e.clientY > rect.top + rect.height / 2;
        targetRule.classList.add(after ? 'drag-over-bottom' : 'drag-over-top');
        parent.insertBefore(dragRule, after ? targetRule.nextSibling : targetRule);
    });

    document.addEventListener('drop', function(e) {
        if (!dragRule) return;
        e.preventDefault();
        e.stopPropagation();
    }, true);

    function finishRule() {
        clearRuleHints();
        if (dragRule) {
            dragRule.classList.remove('dragging');
            dragRule.removeAttribute('draggable');
        }
        dragRule = null;
        markDirty();
    }

    document.addEventListener('dragend', function(e) {
        if (!dragRule) return;
        finishRule();
    });

    document.addEventListener('mouseup', function() {
        var rules = document.querySelectorAll('#mt_modal .mt-rule[draggable="true"]:not(.dragging)');
        for (var i = 0; i < rules.length; i++) rules[i].removeAttribute('draggable');
    }, true);
})();


// Перехватываем изменения полей — метим dirty + live-recount
document.addEventListener('input', function(e) {
    var t = e.target;
    if (!t || !t.closest || !t.closest('#mt_modal')) return;
    if (t.id === 'mt_group_search' || t.id === 'mt_group_filter' || t.closest('#routing_modal')) return;
    MT_STATE.dirty = true;
}, true);
document.addEventListener('change', function(e) {
    var t = e.target;
    if (!t || !t.closest || !t.closest('#mt_modal')) return;
    if (t.id === 'mt_group_search' || t.id === 'mt_group_filter' || t.closest('#routing_modal')) return;
    MT_STATE.dirty = true;
    // toggle группы/правила → применить CSS-класс
    if (t.matches('.mt-group-enable')) {
        var card = t.closest('.mt-group-card');
        if (card) card.classList.toggle('mt-group-off', !t.checked);
    } else if (t.matches('.mt-rule-enable')) {
        var rl = t.closest('.mt-rule');
        if (rl) rl.classList.toggle('mt-rule-off', !t.checked);
    }
}, true);

// Клики: добавить правило, удалить правило/группу, свернуть группу
document.addEventListener('click', function(e) {
    var t = e.target;
    if (!t || !t.closest || !t.closest('#mt_modal')) return;

    var ar = t.closest('.mt-add-rule');
    if (ar) {
        var card = ar.closest('.mt-group-card');
        if (!card) return;
        _mtCollectFromUi();
        var gid = card.getAttribute('data-gid');
        var g = null;
        for (var i = 0; i < (MT_STATE.config.groups || []).length; i++) {
            if (MT_STATE.config.groups[i].id === gid) { g = MT_STATE.config.groups[i]; break; }
        }
        if (!g) return;
        g.rules = g.rules || [];
        g.rules.unshift({ id: _mtNewId(), name: '', type: 'namespace', rule: '', enable: true });
        g.collapsed = false; if (typeof MT_UI !== "undefined" && MT_UI.collapsed) MT_UI.collapsed[g.id] = false;
        mtRenderGroups();
        MT_STATE.dirty = true;
        var addedCard = document.querySelector('.mt-group-card[data-gid="' + gid + '"]');
        var addedInput = addedCard && addedCard.querySelector('.mt-rule-input');
        if (addedInput) addedInput.focus();
        return;
    }

    var rd = t.closest('.rt-rule-del');
    if (rd) {
        var rEl = rd.closest('.mt-rule');
        if (!rEl) return;
        var gcard = rEl.closest('.mt-group-card');
        if (!gcard) return;
        _mtCollectFromUi();
        var gid2 = gcard.getAttribute('data-gid');
        var rid = rEl.getAttribute('data-rid');
        var g2 = null;
        for (var j = 0; j < (MT_STATE.config.groups || []).length; j++) {
            if (MT_STATE.config.groups[j].id === gid2) { g2 = MT_STATE.config.groups[j]; break; }
        }
        if (!g2) return;
        g2.rules = (g2.rules || []).filter(function(r) { return r.id !== rid; });
        if (!g2.rules.length) g2.rules = [{ id: _mtNewId(), name: '', type: 'namespace', rule: '', enable: true }];
        mtRenderGroups();
        MT_STATE.dirty = true;
        return;
    }

    var gd = t.closest('.mt-group-card .rt-group-del');
    if (gd) {
        var card2 = gd.closest('.mt-group-card');
        if (!card2) return;
        var gid3 = card2.getAttribute('data-gid');
        var gName = (card2.querySelector('.mt-group-name') || {}).value || gid3;
        if (!confirm('Удалить группу "' + gName + '"?')) return;
        _mtCollectFromUi();
        MT_STATE.config.groups = (MT_STATE.config.groups || []).filter(function(g) { return g.id !== gid3; });
        mtRenderGroups();
        MT_STATE.dirty = true;
        return;
    }

    var gt = t.closest('.mt-group-toggle');
    if (gt) {
        var card3 = gt.closest('.mt-group-card');
        if (!card3) return;
        var gid4 = card3.getAttribute('data-gid');
        if (!gid4) return;
        /* FIX: перед rerender сохранить введённый в поля текст.
           Если collect не удался (рассинхрон DOM/config, битые группы) —
           НЕ перерисовываем, иначе потеряем правки пользователя. */
        if (typeof _mtCollectFromUi === 'function') {
            if (!_mtCollectFromUi()) return;
        }
        MT_UI.collapsed[gid4] = !MT_UI.collapsed[gid4];
        mtRenderGroups();
        return;
    }
}, true);

async function routingExport() {
    if (!window.__rt_get_data) { alert('Модуль маршрутизации ещё не загружен'); return; }
    var data = window.__rt_get_data();
    var resp;
    try {
        resp = await fetch('/api/routing_export', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({groups: data})
        });
    } catch (e) { alert('Сеть: ' + e.message); return; }
    if (!resp.ok) { alert('Ошибка экспорта: HTTP ' + resp.status); return; }
    var blob = await resp.blob();
    var fname = backupDownloadFilename(resp, 'optiray_routing.xlsx');
    var url = URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url; a.download = fname;
    document.body.appendChild(a); a.click(); document.body.removeChild(a);
    setTimeout(function(){ try { URL.revokeObjectURL(url); } catch (e) {} }, 3000);
}

async function routingImportFile(inp) {
    if (!inp || !inp.files || !inp.files[0]) return;
    var file = inp.files[0];
    if (!confirm('Импортировать правила из "' + file.name + '"?\n\n'
                 + 'Все текущие правила в этом окне будут заменены.')) {
        inp.value = ''; return;
    }
    var b64;
    try {
        b64 = await new Promise(function(resolve, reject) {
            var r = new FileReader();
            r.onload = function() {
                var s = String(r.result || '');
                var c = s.indexOf(',');
                resolve(c >= 0 ? s.slice(c + 1) : s);
            };
            r.onerror = function() { reject(new Error('FileReader error')); };
            r.readAsDataURL(file);
        });
    } catch (e) { alert('Не удалось прочитать файл: ' + e.message); inp.value = ''; return; }
    var resp;
    try {
        resp = await fetch('/api/routing_import', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({file_b64: b64})
        });
    } catch (e) { alert('Сеть: ' + e.message); inp.value = ''; return; }
    var payload = {};
    try { payload = await resp.json(); } catch (e) {}
    inp.value = '';
    if (!resp.ok || !payload || !payload.ok) {
        alert('Ошибка импорта: ' + ((payload && payload.error) || ('HTTP ' + resp.status)));
        return;
    }
    if (!window.__rt_set_data) { alert('Модуль маршрутизации не загружен'); return; }
    window.__rt_set_data(payload.groups || {});
    var total = 0;
    ['block','direct','proxy'].forEach(function(c){
        total += ((payload.groups || {})[c] || []).reduce(function(n, g){
            return n + ((g.rules || []).length);
        }, 0);
    });
    alert('Импорт выполнен: ' + total + ' правил(а).\n\n'
          + 'Не забудьте нажать «Сохранить», чтобы применить.');
}


/* ==== Routing: синхронизация видимости URL-поля ==== */
function __routingSyncUrlField() {
    var f = document.getElementById('routing_url_field');
    var engineBlock = document.getElementById('routing_engine_block');
    var radios = document.querySelectorAll('input[name="routing_use_external"]');
    var val = '1';
    for (var i = 0; i < radios.length; i++) {
        if (radios[i].checked) { val = radios[i].value; break; }
    }
    var isExternal = (val === '1');

    /* URL-поле видно только при внешней маршрутизации */
    if (f) f.style.display = isExternal ? '' : 'none';

    /* Блок выбора движка — только при встроенной */
    if (engineBlock) engineBlock.style.display = isExternal ? 'none' : '';

    /* Блок «Сохранять бэкапы» — только когда выбран OptiRout И встроенная */
    var mtBk = document.getElementById('optirout_save_backups_block');
    if (mtBk) {
        var mtRadio = document.querySelector('input[name="routing_engine"][value="optirout"]');
        var isMt = mtRadio && mtRadio.checked;
        mtBk.style.display = (!isExternal && isMt) ? '' : 'none';
    }
}

/* Реакция на смену движка */
document.addEventListener('change', function(e) {
    var t = e.target;
    if (!t || !t.matches) return;
    if (t.matches('input[name="routing_engine"]')) {
        __routingSyncUrlField();
    }
}, true);

document.addEventListener('DOMContentLoaded', function() {
    __routingSyncUrlField();
});

function closeRoutingModal() {
    var m = document.getElementById('routing_modal');
    if (m) m.style.display = 'none';
}


window.addEventListener('DOMContentLoaded', function() {
    restorePings();
    applySubsCollapseState();
    document.querySelectorAll('.sub-section').forEach(function(sec) {
        var id = sec.id.split('-')[2];
        if (localStorage.getItem('sub_collapsed_' + id) === '1') {
            sec.classList.add('collapsed');
            var b = document.getElementById('collapse_btn_' + id);
            if (b) renderSubscriptionToggle(sec, b, true);
            var m = document.getElementById('no_favs_msg_' + id);
            if (m) m.style.display = sec.querySelectorAll('[data-fav="1"], [data-active="1"]').length ? 'none' : 'block';
        }
    });
    // Показ/скрытие плавающих кнопок «Вверх» и «Тест» при скролле
    window.addEventListener('scroll', function() {
        var visible = window.scrollY > 50;
        var b = document.getElementById('scroll_top_btn');
        if (b) visible ? b.classList.add('visible') : b.classList.remove('visible');
        var t = document.getElementById('test_float_btn');
        if (t) visible ? t.classList.add('visible') : t.classList.remove('visible');
    });

    var dragged = null;
    function finishDrag(row) {
        row.classList.remove('dragging');
        row.removeAttribute('draggable');
        dragged = null;
        var ids = Array.from(document.getElementById('subs_container').querySelectorAll('.sub-row'))
            .map(function(r) { return parseInt(r.dataset.subId); });
        var sc = document.getElementById('servers_container');
        ids.forEach(function(id) {
            var s = document.getElementById('sub-section-' + id);
            if (s) sc.appendChild(s);
        });
        fetch('/api/reorder_subs', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ order: ids })
        });
    }
    function handleDragMove(clientY) {
        if (!dragged) return;
        var siblings = Array.from(document.getElementById('subs_container').querySelectorAll('.sub-row:not(.dragging)'));
        var next = null;
        for (var i = 0; i < siblings.length; i++) {
            var rect = siblings[i].getBoundingClientRect();
            if (clientY < rect.top + rect.height / 2) { next = siblings[i]; break; }
        }
        dragged.parentNode.insertBefore(dragged, next);
    }
    document.querySelectorAll('.sub-row').forEach(function(row) {
        var h = row.querySelector('.drag-handle');
        if (!h) return;
        h.onmousedown = function() { row.setAttribute('draggable', 'true'); };
        h.onmouseup = function() { row.removeAttribute('draggable'); };
        row.ondragstart = function() { dragged = row; row.classList.add('dragging'); };
        row.ondragend = function() { finishDrag(row); };
        row.ondragover = function(e) { e.preventDefault(); if (dragged && dragged !== row) handleDragMove(e.clientY); };
        h.addEventListener('touchstart', function(e) { e.preventDefault(); dragged = row; row.classList.add('dragging'); }, { passive: false });
    });
    document.addEventListener('touchmove', function(e) {
        if (!dragged) return;
        e.preventDefault();
        handleDragMove(e.touches[0].clientY);
    }, { passive: false });
    document.addEventListener('touchend', function() {
        if (!dragged) return;
        finishDrag(dragged);
    });
});




/* ==== OPTIRAY_CLEAN_BLOCK_B (bottom of JS) ==== */
(function() {
    'use strict';

    /* Отмена — просто прячем модалку (по таймеру всё равно закроется) */
    window.cancelWarmup = function() {
        if (window.hideWaitModal) window.hideWaitModal();
    };

    window.connectServer = async function(btn) {
        var card = btn.closest('.server-card');
        if (!card) return;
        var orig = btn.innerText;
        btn.innerText = 'Подключение...';
        btn.disabled = true;
        if (window.showWaitModal) {
            window.showWaitModal('Подключение к серверу...', 'Применение конфигурации');
        }
        try {
            var fd = new URLSearchParams();
            fd.append('sid', card.dataset.sid);
            var res = await fetch('/select', { method: 'POST', body: fd });
            var payload = {};
            try { payload = await res.json(); } catch(e) {}

            if (!res.ok) {
                if (window.hideWaitModal) window.hideWaitModal();
                alert(payload.error || 'Не удалось подключиться к серверу');
                btn.innerText = orig;
                btn.disabled = false;
                return;
            }

            /* Обновляем UI */
            var powerToggle=document.getElementById('optiray_power');if(powerToggle){powerToggle.checked=true;powerToggle.disabled=false;syncPowerButtons();}
            document.querySelectorAll('.server-card').forEach(function(c) {
                c.classList.remove('active-card');
                c.dataset.active = '0';
                var s = c.querySelector('.action-slot');
                if (s) s.innerHTML = '<button type="button" onclick="connectServer(this)" class="btn-connect text-style-7">Подключить</button>';
            });
            card.classList.add('active-card');
            card.dataset.active = '1';
            card.parentNode.insertBefore(card, card.parentNode.firstChild);
            var slot = card.querySelector('.action-slot');
            if (slot) slot.innerHTML = '<div class="badge-connected text-style-7">Подключено</div>';

            var sec = card.closest('[id^="sub-section-"]');
            if (sec) {
                var asn = document.getElementById('active-sub-name');
                if (asn) asn.innerText = sec.dataset.subname || 'Подписка';
            }
            var asrv = document.getElementById('active-srv-name');
            if (asrv) asrv.innerText = card.dataset.name || '';
            var dot = document.getElementById('active-status-dot');
            if (dot) { dot.classList.remove('disconnected'); dot.classList.add('connected'); }

            var coreBadge = document.getElementById('active-core-badge');
            if (coreBadge) {
                var protoLow = (card.dataset.protocol || '').toLowerCase();
                if (protoLow === 'hysteria' || protoLow === 'hysteria2') {
                    coreBadge.innerText = 'Hysteria';
                    coreBadge.className = 'core-badge core-hysteria';
                } else {
                    coreBadge.innerText = 'Xray';
                    coreBadge.className = 'core-badge core-xray';
                }
                coreBadge.style.display = '';
            }

            /* Проверка выполнена роутером через выбранный SOCKS-прокси.
               Запрос из браузера при раздельной маршрутизации не проверяет этот сервер. */
            if (window.hideWaitModal) window.hideWaitModal();
            if (payload.warning) {
                alert(payload.warning);
            } else if (payload.verified === false) {
                alert('Прокси запущен, но тестовый адрес не ответил через него.\n\n' +
                      'Проверка не подтверждена. Это не означает, что сервер нерабочий.');
            }
        } catch(e) {
            if (window.hideWaitModal) window.hideWaitModal();
            alert('Ошибка подключения: '+e.message);
            btn.innerText = orig;
            btn.disabled = false;
        }
    };

    window.autoSelectSub = async function(subId, btn) {
        var orig = btn.innerText;
        btn.innerText = 'Подключение...';
        btn.disabled = true;
        if (window.showWaitModal) {
            window.showWaitModal('Автовыбор сервера...', 'Замер задержек и подключение');
        }
        try {
            var fd = new URLSearchParams();
            fd.append('sub_id', subId);
            var res = await fetch('/api/auto_select', { method: 'POST', body: fd });
            var payload = {};
            try { payload = await res.json(); } catch(e) {}

            if (!res.ok) {
                if (window.hideWaitModal) window.hideWaitModal();
                if (window.console) console.log('optiray: ошибка автовыбора:', payload.error || 'не удалось');
                btn.innerText = orig;
                btn.disabled = false;
                return;
            }

            if (payload.pings && typeof payload.pings === 'object') {
                try {
                    /* 1. Сбрасываем бейджи у ВСЕХ карточек этой подписки в DOM:
                       старые значения не должны путать пользователя. */
                    document.querySelectorAll('.server-card.sub-group-' + subId).forEach(function(c) {
                        var b = c.querySelector('.ping-badge');
                        if (b) { b.innerText = '-- ms'; b.className = 'ping-badge'; }
                    });
                    /* 2. Удаляем старые пинги этой подписки из кэша. */
                    var cache = JSON.parse(localStorage.getItem('optiray_ping_cache') || '{}');
                    document.querySelectorAll('.server-card.sub-group-' + subId).forEach(function(c) {
                        if (c.dataset.sid) delete cache[c.dataset.sid];
                    });
                    /* 3. Добавляем только свежие пинги из ответа. */
                    Object.keys(payload.pings).forEach(function(sid) {
                        var v = payload.pings[sid];
                        if (v && v.text && v.cls) {
                            cache[sid] = { text: v.text, cls: v.cls, min: v.min || 0, max: v.max || 0 };
                        }
                    });
                    localStorage.setItem('optiray_ping_cache', JSON.stringify(cache));
                } catch(e) {}
            }

            if (window.hideWaitModal) window.hideWaitModal();
            if (payload.warning) alert(payload.warning);
            window.location.reload();
        } catch(e) {
            if (window.hideWaitModal) window.hideWaitModal();
            if (window.console) console.log('optiray: сетевая ошибка (автовыбор):', e.message);
            btn.innerText = orig;
            btn.disabled = false;
        }
    };

    if (window.console && console.log) {
        console.log('optiray: clean override (Block A + B) installed');
    }
})();






/* ==== OptiRout: вставка группы через модалку ==== */
window.mtOpenPasteModal = function(btn) {
    var m = document.getElementById('paste_group_modal');
    var ta = document.getElementById('paste_group_textarea');
    var hint = document.getElementById('paste_group_hint');
    if (!m || !ta) { alert('Не удалось открыть окно вставки.'); return; }
    ta.value = '';
    if (hint) hint.textContent = 'Первая строка — название группы, далее правила. Ctrl+Enter — добавить.';
    window.__mt_paste_ctx = { btn: btn, isMt: true };
    m.style.display = 'flex';
    setTimeout(function() { try { ta.focus(); } catch(e) {} }, 60);
};
window.applyPasteGroupMt = function() {
    var ctx = window.__mt_paste_ctx;
    var ta = document.getElementById('paste_group_textarea');
    if (!ta || !ctx || !ctx.isMt) { window.closePasteGroupModal(); return; }
    var raw = ta.value || '';
    if (!String(raw).trim()) {
        var hint = document.getElementById('paste_group_hint');
        if (hint) hint.textContent = 'Пусто — нечего добавлять.';
        return;
    }
    var lines = String(raw).split(/\r?\n/);
    var name = '';
    var rules = [];
    for (var i = 0; i < lines.length; i++) {
        var v = String(lines[i] || '').replace(/^\s+|\s+$/g, '');
        if (!v) continue;
        if (v.charAt(0) === '#') continue;
        if (!name) { name = v; continue; }
        rules.push(v);
    }
    if (!name && !rules.length) {
        var hint2 = document.getElementById('paste_group_hint');
        if (hint2) hint2.textContent = 'Не удалось распознать формат.';
        return;
    }
    if (typeof _mtCollectFromUi === 'function') _mtCollectFromUi(true);
    var cfg = MT_STATE.config;
    if (!cfg) { window.closePasteGroupModal(); return; }
    cfg.groups = cfg.groups || [];
    var newRules = [];
    for (var j = 0; j < rules.length; j++) {
        var rv = rules[j];
        var type = 'namespace';
        if (typeof window.mtDetectRuleType === 'function') {
            var detected = window.mtParseImportedRule(rv);
                rv = detected.rule; type = detected.type;
        }
        newRules.push({ id: _mtNewId(), name: '', type: type, rule: rv, enable: true });
    }
    if (!newRules.length) newRules.push({ id: _mtNewId(), name: '', type: 'namespace', rule: '', enable: true });
    var iface0 = (MT_STATE.interfaces && MT_STATE.interfaces[0])
        ? (_mtIfaceName(MT_STATE.interfaces[0]) || 't2s0') : 't2s0';
    var newGroup = {
        id: _mtNewId(), name: name || 'Новая группа', color: randomRoutingGroupColor(),
        interface: iface0, enable: true, rules: newRules
    };
    cfg.groups.unshift(newGroup);
    mtRenderGroups();
    MT_STATE.dirty = true;
    window.closePasteGroupModal();
    _mtSetStatus('Вставлена группа "' + (name || 'Новая группа') + '" (' + rules.length + ' правил)', 'ok');
    if (typeof window.__mtScanDups === 'function') setTimeout(window.__mtScanDups, 150);
};
/* ==== Routing textarea: remember height (v2) ==== */
(function() {
    'use strict';
    var NAMES = ['routing_block_rules', 'routing_direct_rules', 'routing_proxy_rules'];
    function key(n) { return 'optiray_ta_h_' + n; }

    function getH(el) {
        var h = parseInt(el.style.height || '', 10);
        return (h >= 40 && h <= 3000) ? h : 0;
    }
    function restoreOne(el, name) {
        try {
            var v = localStorage.getItem(key(name));
            if (!v) return;
            var h = parseInt(v, 10);
            if (h >= 40 && h <= 3000) el.style.height = h + 'px';
        } catch(e) {}
    }
    function restore() {
        for (var k = 0; k < NAMES.length; k++) {
            var el = document.querySelector('textarea[name="' + NAMES[k] + '"]');
            if (el) restoreOne(el, NAMES[k]);
        }
    }
    function saveOne(el, name) {
        try {
            var h = getH(el);
            if (!h) return;
            if (localStorage.getItem(key(name)) !== String(h)) {
                localStorage.setItem(key(name), h);
            }
        } catch(e) {}
    }
    function bind() {
        for (var k = 0; k < NAMES.length; k++) {
            (function(name) {
                var el = document.querySelector('textarea[name="' + name + '"]');
                if (!el || el.__optiray_h_bound) return;
                el.__optiray_h_bound = true;

                var poll = null;
                function startPoll() {
                    if (poll) return;
                    poll = setInterval(function() { saveOne(el, name); }, 200);
                }
                function stopPoll() {
                    if (poll) { clearInterval(poll); poll = null; }
                    /* финальный снимок — после того, как браузер применит размер */
                    setTimeout(function() { saveOne(el, name); }, 40);
                }

                el.addEventListener('mousedown', startPoll);
                el.addEventListener('touchstart', startPoll, { passive: true });
                document.addEventListener('mouseup', stopPoll);
                document.addEventListener('touchend', stopPoll);
                el.addEventListener('blur', function() { saveOne(el, name); });

                /* ResizeObserver как страховка: если реально изменился размер */
                if (window.ResizeObserver) {
                    try {
                        new ResizeObserver(function() {
                            /* только когда элемент реально видим */
                            if (el.offsetParent !== null) saveOne(el, name);
                        }).observe(el);
                    } catch(e) {}
                }
            })(NAMES[k]);
        }
    }
    function kick() { restore(); bind(); }
    window._optirayRoutingKick = kick;

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() { setTimeout(kick, 50); });
    } else {
        setTimeout(kick, 50);
    }
    var _origOpen = window.openSettingsModal;
    window.openSettingsModal = function() {
        if (typeof _origOpen === 'function') {
            try { _origOpen.apply(this, arguments); } catch(e) {}
        }
        setTimeout(kick, 60);
    };
})();


/* ==== Routing: группы правил (шаг 1) ==== */
(function() {
    'use strict';
    if (window.__RT_GROUPS_INSTALLED) return;
    window.__RT_GROUPS_INSTALLED = true;

    var CATS = ['block', 'direct', 'proxy'];
    var DATA = { block: [], direct: [], proxy: [] };

    function uid() { return 'g' + Math.random().toString(36).slice(2, 8); }
    function esc(s) {
        return String(s == null ? '' : s)
            .replace(/&/g, '&amp;').replace(/</g, '&lt;')
            .replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }
    function findGroup(cat, gid) {
        var list = DATA[cat] || [];
        for (var i = 0; i < list.length; i++) if (list[i].id === gid) return list[i];
        return null;
    }

    function normRuleState(r) {
        if (r && typeof r === 'object') {
            return {value: String(r.value == null ? '' : r.value), enabled: r.enabled !== false};
        }
        return {value: String(r == null ? '' : r), enabled: true};
    }
    function ruleHtml(cat, gid, v, enabled) {
        var off = (enabled === false) ? ' rt-rule-off' : '';
        var chk = (enabled === false) ? '' : ' checked';
        return '<div class="rt-rule' + off + '" data-cat="' + cat + '" data-gid="' + gid + '">' +
            '<span class="rt-rule-drag" title="Перетащить">' + OPTIRAY_ROUTING_ICONS.drag + '</span>' +
            '<label class="rt-check" title="Включить / отключить правило">' +
                '<input type="checkbox" class="rt-rule-enabled"' + chk + '>' +
            '</label>' +
            '<input type="text" class="rt-rule-input" value="' + esc(v) +
                '" placeholder="example.com / domain:... / ip:...">' +
            '<div class="mt-rule-controls xr-rule-controls" role="group" aria-label="Действия правила">' +
            '<button type="button" class="rt-icon-btn rt-rule-del" title="Удалить">' + OPTIRAY_ROUTING_ICONS.delete + '</button>' +
            '</div>' +
        '</div>';
    }
    function groupHtml(cat, g) {
        var collapsed = g.collapsed ? ' collapsed' : '';
        var enabled = (g.enabled === false) ? false : true;
        var grp_off = enabled ? '' : ' rt-group-off';
        var grp_chk = enabled ? ' checked' : '';
        var _rules = (g.rules && g.rules.length) ? g.rules : [{value:'', enabled:true}];
        var rulesHtml = '';
        for (var i = 0; i < _rules.length; i++) {
            var _r = normRuleState(_rules[i]);
            rulesHtml += ruleHtml(cat, g.id, _r.value, _r.enabled);
        }
        var SVG_LIST = OPTIRAY_ROUTING_ICONS.bulk;
        var SVG_DEL = OPTIRAY_ROUTING_ICONS.delete;
        var SVG_TOG = OPTIRAY_ROUTING_ICONS.toggle;
        var SVG_COPY = OPTIRAY_ROUTING_ICONS.copy;
        return '<div class="rt-group-card' + collapsed + grp_off + '" style="--rule-group-color:' + esc(g.color || ({block:'#ed4046',direct:'#278c45',proxy:'#1971c2'}[cat])) + '" data-cat="' + cat + '" data-gid="' + g.id + '">' +
            '<div class="rt-group-head">' +
                '<span class="rt-group-drag" title="Перетащить">' + OPTIRAY_ROUTING_ICONS.drag + '</span>' +
                '<label class="rt-check rt-check-group" title="Включить / отключить группу">' +
                    '<input type="checkbox" class="rt-group-enabled"' + grp_chk + '>' +
                '</label>' +
                '<input type="color" class="xr-group-color" aria-label="Цвет группы" value="' + esc(g.color || ({block:'#ed4046',direct:'#278c45',proxy:'#1971c2'}[cat])) + '">' +
                '<span class="xr-category-dot" title="' + ({block:'Блокировать',direct:'Напрямую',proxy:'Через прокси'}[cat]) + '"></span>' +
                '<input type="text" class="rt-group-title" value="' + esc(g.title || '') +
                    '" placeholder="Название группы">' +
                '<div class="rt-group-actions"><button type="button" class="rt-icon-btn rt-add-rule" title="Добавить правило" aria-label="Добавить правило"><svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14"/></svg></button>' +
                '<button type="button" class="rt-icon-btn rt-group-bulk" title="Добавить списком">' + SVG_LIST + '</button>' +
                '<button type="button" class="rt-icon-btn rt-group-copy" title="Копировать группу (имя + правила)">' + SVG_COPY + '</button>' +
                '<button type="button" class="rt-icon-btn rt-group-del" title="Удалить группу">' + SVG_DEL + '</button>' +
                '</div>' +
                '<span class="xr-group-count">' + _rules.filter(function(r){return normRuleState(r).value.trim();}).length + ' правил</span>' +
                '<button type="button" class="rt-icon-btn rt-group-toggle" title="Свернуть">' + SVG_TOG + '</button>' +
            '</div>' +
            '<div class="rt-group-body">' +
                '<div class="rt-rules">' + rulesHtml + '</div>' +
                
            '</div>' +
        '</div>';

    }

    var _TA_BY_CAT = {
        block:  'routing_block_rules',
        direct: 'routing_direct_rules',
        proxy:  'routing_proxy_rules'
    };

    function _textareaRulesFor(cat) {
        var name = _TA_BY_CAT[cat];
        if (!name) return 0;
        var el = document.querySelector('textarea[name="' + name + '"]');
        if (!el) return 0;
        var lines = String(el.value || '').split(/\r?\n/);
        var n = 0;
        for (var i = 0; i < lines.length; i++) {
            var v = String(lines[i] || '').replace(/^\s+|\s+$/g, '');
            if (!v) continue;
            if (v.charAt(0) === '#') continue;
            n++;
        }
        return n;
    }

    function _catCounts(cat) {
        var list = DATA[cat] || [];
        var gTotal = list.length;
        var gOn = 0;
        var rTotal = 0, rOn = 0;
        for (var i = 0; i < list.length; i++) {
            var g = list[i];
            var gEnabled = (g.enabled !== false);
            if (gEnabled) gOn++;
            var rs = g.rules || [];
            for (var j = 0; j < rs.length; j++) {
                var r = normRuleState(rs[j]);
                if (!r.value) continue;
                rTotal++;
                if (gEnabled && r.enabled) rOn++;
            }
        }
        /* Строки в большом textarea — тоже правила, всегда «включены». */
        var taN = _textareaRulesFor(cat);
        rTotal += taN;
        rOn += taN;
        return {gTotal:gTotal, gOn:gOn, rTotal:rTotal, rOn:rOn, rTa:taN};
    }

    function updateCatCounts() {
        for (var i = 0; i < CATS.length; i++) {
            var cat = CATS[i];
            var el = document.getElementById('rt_count_' + cat);
            if (!el) continue;
            var c = _catCounts(cat);
            var allOn = (c.gOn === c.gTotal) && (c.rOn === c.rTotal);
            var text;
            if (allOn) {
                text = 'групп: ' + c.gTotal + ' · правил: ' + c.rTotal;
            } else {
                text = 'групп: ' + c.gOn + '/' + c.gTotal +
                       ' · правил: ' + c.rOn + '/' + c.rTotal;
            }
            el.textContent = text;
            el.classList.toggle('rt-cat-count-off', c.rTotal > 0 && c.rOn === 0);
            el.title = 'Групп активно: ' + c.gOn + ' / ' + c.gTotal
                     + ' · Правил всего: ' + c.rTotal
                     + ' (в группах: ' + (c.rTotal - c.rTa) + ', в поле: ' + c.rTa + ')'
                     + ' · Активных: ' + c.rOn;
        }
    }

    function renderAll() {
        for (var c = 0; c < CATS.length; c++) {
            var cat = CATS[c];
            var box = document.getElementById('rt_groups_' + cat);
            if (!box) continue;
            var list = DATA[cat] || [];
            var html = '';
            for (var i = 0; i < list.length; i++) html += groupHtml(cat, list[i]);
            box.innerHTML = html;
        }
        updateCatCounts();
        if(window.syncXraySplitFields)window.syncXraySplitFields();
        if(window.validateXrayGeoReferences)window.validateXrayGeoReferences();
        if(window.filterXrayGroups)window.filterXrayGroups();
    }

    function collectFromUI() {
        for (var c = 0; c < CATS.length; c++) {
            var cat = CATS[c];
            var box = document.getElementById('rt_groups_' + cat);
            if (!box) continue;
            var newList = [];
            var cards = box.querySelectorAll('.rt-group-card');
            for (var i = 0; i < cards.length; i++) {
                var card = cards[i];
                var gid = card.getAttribute('data-gid');
                var g = findGroup(cat, gid) || { id: gid, title: '', rules: [], collapsed: false };
                var titleEl = card.querySelector('.rt-group-title');
                g.title = titleEl ? titleEl.value : (g.title || '');
                var colorEl = card.querySelector('.xr-group-color');
                if (colorEl) g.color = colorEl.value;
                g.collapsed = card.classList.contains('collapsed');
                var gEn = card.querySelector('.rt-group-enabled');
                g.enabled = gEn ? !!gEn.checked : (g.enabled !== false);
                var ruleEls = card.querySelectorAll('.rt-rule');
                var rules = [];
                for (var r = 0; r < ruleEls.length; r++) {
                    var rEl = ruleEls[r];
                    var inp = rEl.querySelector('.rt-rule-input');
                    var chk = rEl.querySelector('.rt-rule-enabled');
                    var v = (inp ? inp.value : '').replace(/^\s+|\s+$/g, '');
                    if (!v) continue;
                    rules.push({value: v, enabled: chk ? !!chk.checked : true});
                }
                g.rules = rules;
                newList.push(g);
            }
            DATA[cat] = newList;
        }
    }

    function onAddGroup(cat) {
        collectFromUI();
        DATA[cat] = DATA[cat] || [];
        DATA[cat].push({ id: uid(), title: 'Новая группа', color: randomRoutingGroupColor(), rules: [''], collapsed: false });
        renderAll();
    }
    function onDelGroup(cat, gid) {
        if (!confirm('Удалить группу со всеми правилами?')) return;
        collectFromUI();
        DATA[cat] = (DATA[cat] || []).filter(function(g) { return g.id !== gid; });
        renderAll();
    }
    function onToggleGroup(cat, gid) {
        collectFromUI();
        var g = findGroup(cat, gid);
        if (!g) return;
        g.collapsed = !g.collapsed;
        renderAll();
    }
    function onAddRule(cat, gid) {
        collectFromUI();
        var g = findGroup(cat, gid);
        if (!g) return;
        g.rules = g.rules || [];
        g.rules.unshift({value:'', enabled:true});
        g.collapsed = false;
        renderAll();
        var box = document.getElementById('rt_groups_' + cat);
        if (box) {
            var card = box.querySelector('.rt-group-card[data-gid="' + gid + '"]');
            if (card) {
                var inputs = card.querySelectorAll('.rt-rule-input');
                if (inputs.length) inputs[0].focus();
            }
        }
    }
    function onDelRule(cat, gid, idx) {
        collectFromUI();
        var g = findGroup(cat, gid);
        if (!g || !g.rules) return;
        g.rules.splice(idx, 1);
        if (g.rules.length === 0) g.rules = [{value:'', enabled:true}];
        renderAll();
    }

    function _triggerDupsScan(markDirty) {
        setTimeout(function() {
            var el = document.querySelector('#routing_modal .rt-rule-input');
            if (el) { try { var event=new Event('input', { bubbles: true }); event.optirayValidationOnly=markDirty===false; el.dispatchEvent(event); } catch(e) {} }
            var saveBtn = document.getElementById('routing_save_btn');
            if (saveBtn && !saveBtn.classList.contains('routing-dirty')) {
                saveBtn.classList.add('routing-dirty');
            }
        }, 0);
    }

    function _copyTextToClipboard(text) {
        if (navigator.clipboard && window.isSecureContext) {
            return navigator.clipboard.writeText(text);
        }
        return new Promise(function(resolve, reject) {
            try {
                var ta = document.createElement('textarea');
                ta.value = text;
                ta.setAttribute('readonly', '');
                ta.style.position = 'fixed';
                ta.style.top = '-1000px';
                ta.style.left = '-1000px';
                document.body.appendChild(ta);
                ta.select();
                ta.setSelectionRange(0, ta.value.length);
                var ok = document.execCommand('copy');
                document.body.removeChild(ta);
                ok ? resolve() : reject(new Error('execCommand copy failed'));
            } catch (e) { reject(e); }
        });
    }

    function _flashBtn(btn, ok) {
        if (!btn) return;
        var orig = btn.innerHTML;
        var svgOk  = '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="20 6 9 17 4 12"/></svg>';
        var svgErr = '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>';
        btn.innerHTML = ok ? svgOk : svgErr;
        btn.classList.add(ok ? 'rt-copy-ok' : 'rt-copy-err');
        setTimeout(function() {
            btn.innerHTML = orig;
            btn.classList.remove('rt-copy-ok', 'rt-copy-err');
        }, 900);
    }

    function onCopyGroup(cat, gid, btn) {
        collectFromUI();
        var g = findGroup(cat, gid);
        if (!g) return;
        var lines = [];
        lines.push(String(g.title || '').trim());
        var rs = g.rules || [];
        for (var i = 0; i < rs.length; i++) {
            var r = normRuleState(rs[i]);
            var v = String(r.value || '').trim();
            if (v) lines.push(v);
        }
        var text = lines.join('\n');
        _copyTextToClipboard(text).then(function() {
            _flashBtn(btn, true);
            if (window.console) console.log('optiray: скопировано в буфер:\n' + text);
        }).catch(function(e) {
            _flashBtn(btn, false);
            var msg = 'Не удалось скопировать в буфер обмена.\n\n' + (e && e.message ? e.message : e);
            if (window.console) console.warn('optiray copy:', e);
            alert(msg);
        });
    }

    function _flashPasteBtn(btn, ok) {
        if (!btn) return;
        btn.classList.add(ok ? 'rt-paste-ok' : 'rt-paste-err');
        setTimeout(function() {
            btn.classList.remove('rt-paste-ok', 'rt-paste-err');
        }, 900);
    }

    function _readClipboardText() {
        /* 1) Clipboard API — работает в secure context (HTTPS/localhost). */
        if (navigator.clipboard && navigator.clipboard.readText) {
            return navigator.clipboard.readText().catch(function () {
                return _pasteViaModal();
            });
        }
        /* 2) execCommand('paste') через скрытый textarea — иногда доступно в HTTP. */
        try {
            var ta = document.createElement('textarea');
            ta.setAttribute('readonly', '');
            ta.style.position = 'fixed';
            ta.style.top = '-1000px';
            ta.style.left = '-1000px';
            document.body.appendChild(ta);
            ta.focus();
            ta.select();
            var ok = document.execCommand('paste');
            var v = ta.value;
            document.body.removeChild(ta);
            if (ok && v) return Promise.resolve(v);
        } catch (e) { /* ignore */ }
        /* 3) Своя модалка с textarea — сохраняет переводы строк. */
        return _pasteViaModal();
    }

    function _pasteViaModal() {
        return new Promise(function (resolve) {
            var m = document.getElementById('paste_group_modal');
            var ta = document.getElementById('paste_group_textarea');
            var hint = document.getElementById('paste_group_hint');
            if (!m || !ta) { resolve(''); return; }
            ta.value = '';
            if (hint) hint.textContent = '';
            window.__paste_group_ctx = { resolve: resolve, btn: null };
            m.style.display = 'flex';
            setTimeout(function () {
                try { ta.focus(); } catch (e) {}
            }, 60);
        });
    }

    function _parsePastedGroup(raw) {
        var lines = String(raw == null ? '' : raw).split(/\r?\n/);
        var title = '';
        var rules = [];
        var seen = {blackhole:1};
        for (var i = 0; i < lines.length; i++) {
            var v = String(lines[i] || '').replace(/^\s+|\s+$/g, '');
            if (!v) continue;
            if (v.charAt(0) === '#') continue;
            if (!title) { title = v; continue; }
            if (seen[v]) continue;
            seen[v] = 1;
            rules.push(v);
        }
        return { title: title, rules: rules };
    }

    function _insertPastedGroup(cat, raw) {
        var p = _parsePastedGroup(raw);
        if (!p.title && !p.rules.length) return false;
        collectFromUI();
        DATA[cat] = DATA[cat] || [];
        var newRules = p.rules.length
            ? p.rules.map(function (v) { return { value: v, enabled: true }; })
            : [{ value: '', enabled: true }];
        var existingGroup = DATA[cat].find(function(g){ return g.title === p.title; });
        if (existingGroup) {
            var replace = document.getElementById('paste_group_modal_replace').checked;
            existingGroup.rules = replace ? newRules : (existingGroup.rules || []).concat(newRules);
            existingGroup.collapsed = false;
        } else {
        DATA[cat].push({
            id: uid(),
            title: p.title || 'Новая группа',
            color: randomRoutingGroupColor(),
            rules: newRules,
            collapsed: false,
            enabled: true
        });
        }
        renderAll();
        _triggerDupsScan();
        var saveBtn = document.getElementById('routing_save_btn');
        if (saveBtn) saveBtn.classList.add('routing-dirty');
        return true;
    }

    async function onPasteGroup(cat, btn) {
        /* Пробуем мгновенное чтение буфера (работает в secure context). */
        var quick = null;
        if (navigator.clipboard && navigator.clipboard.readText) {
            try { quick = await navigator.clipboard.readText(); } catch (e) { quick = null; }
        }
        if (quick && String(quick).trim()) {
            if (_insertPastedGroup(cat, quick)) {
                _flashPasteBtn(btn, true);
                return;
            }
        }
        /* Не удалось — открываем модалку с textarea для ручной вставки. */
        window.__paste_group_ctx = { cat: cat, resolve: null, btn: btn };
        var m = document.getElementById('paste_group_modal');
        var ta = document.getElementById('paste_group_textarea');
        var hint = document.getElementById('paste_group_hint');
        if (!m || !ta) {
            _flashPasteBtn(btn, false);
            alert('Не удалось открыть окно вставки.');
            return;
        }
        ta.value = '';
        if (hint) {
            hint.textContent = 'Совет: после вставки нажмите Ctrl+Enter, чтобы добавить сразу.';
        }
        m.style.display = 'flex';
        setTimeout(function () { try { ta.focus(); } catch (e) {} }, 60);
    }

    window.closePasteGroupModal = function () {
        var m = document.getElementById('paste_group_modal');
        if (m) m.style.display = 'none';
        var ctx = window.__paste_group_ctx;
        window.__paste_group_ctx = null;
        if (ctx && ctx.btn) _flashPasteBtn(ctx.btn, false);
    };

    
/* ==== OptiRout: универсальная обёртка для кнопки "Добавить" в модалке ==== */
window.applyPasteGroupAny = function () {
    var ctx = window.__paste_group_ctx;
    if (ctx && ctx.isMt) {
        /* OptiRout контекст */
        var ta = document.getElementById('paste_group_textarea');
        var hint = document.getElementById('paste_group_hint');
        if (!ta) { window.closePasteGroupModal(); return; }
        var raw = ta.value || '';
        if (!String(raw).trim()) {
            if (hint) hint.textContent = 'Пусто — нечего добавлять.';
            return;
        }
        var lines = String(raw).split(/\r?\n/);
        var name = '';
        var rules = [];
        for (var i = 0; i < lines.length; i++) {
            var v = String(lines[i] || '').replace(/^\s+|\s+$/g, '');
            if (!v) continue;
            if (v.charAt(0) === '#') continue;
            if (!name) { name = v; continue; }
            rules.push(v);
        }
        if (!name && !rules.length) {
            if (hint) hint.textContent = 'Не удалось распознать формат.';
            return;
        }
        if (typeof _mtCollectFromUi === 'function') _mtCollectFromUi(true);
        var cfg = MT_STATE.config;
        if (!cfg) { window.closePasteGroupModal(); return; }
        cfg.groups = cfg.groups || [];
        var newRules = [];
        for (var j = 0; j < rules.length; j++) {
            var rv = rules[j];
            var type = 'namespace';
            if (typeof window.mtDetectRuleType === 'function') {
                var detected = window.mtParseImportedRule(rv);
                rv = detected.rule; type = detected.type;
            }
            newRules.push({ id: _mtNewId(), name: '', type: type, rule: rv, enable: true });
        }
        if (!newRules.length) newRules.push({ id: _mtNewId(), name: '', type: 'namespace', rule: '', enable: true });
        var iface0 = (MT_STATE.interfaces && MT_STATE.interfaces[0])
            ? (_mtIfaceName(MT_STATE.interfaces[0]) || 't2s0') : 't2s0';
        var newGroup = {
            id: _mtNewId(), name: name || 'Новая группа', color: randomRoutingGroupColor(),
            interface: iface0, enable: true, rules: newRules
        };
        cfg.groups.unshift(newGroup);
        mtRenderGroups();
        MT_STATE.dirty = true;
        window.closePasteGroupModal();
        _mtSetStatus('Вставлена группа "' + (name || 'Новая группа') + '" (' + rules.length + ' правил)', 'ok');
        if (typeof window.__mtScanDups === 'function') setTimeout(window.__mtScanDups, 150);
        return;
    }
    /* Routing контекст — вызываем оригинальную функцию */
    if (typeof window._applyPasteGroupOrig === 'function') {
        window._applyPasteGroupOrig();
    }
};
window.applyPasteGroup = function () {
        var ctx = window.__paste_group_ctx;
        if (!ctx) { window.closePasteGroupModal(); return; }
        var ta = document.getElementById('paste_group_textarea');
        if (!ta) { window.closePasteGroupModal(); return; }
        var raw = ta.value || '';
        if (!String(raw).trim()) {
            var hint = document.getElementById('paste_group_hint');
            if (hint) hint.textContent = 'Пусто — нечего добавлять.';
            return;
        }
        var ok = _insertPastedGroup(ctx.cat, raw);
        var btn = ctx.btn;
        window.closePasteGroupModal();
        if (btn) _flashPasteBtn(btn, ok);
    };

    function onBulkAddRules(cat, gid) {
        if (!findGroup(cat, gid)) return;
        window.__rt_bulk_ctx = { cat: cat, gid: gid };
        var m = document.getElementById('bulk_rules_modal');
        var ta = document.getElementById('bulk_rules_textarea');
        var hint = document.getElementById('bulk_rules_hint');
        if (!m || !ta) return;
        ta.value = '';
        if (hint) hint.textContent = '';
        m.style.display = 'flex';
        setTimeout(function() { try { ta.focus(); } catch(e) {} }, 60);
    }

    window.closeBulkRulesModal = function() {
        var m = document.getElementById('bulk_rules_modal');
        if (m) m.style.display = 'none';
        window.__rt_bulk_ctx = null;
    };

    window.applyBulkRules = function() {
        if (document.getElementById('bulk_rules_modal_replace').checked) return window.applyBulkRulesReplace();
        var ctx = window.__rt_bulk_ctx;
        if (!ctx) { window.closeBulkRulesModal(); return; }
        var ta = document.getElementById('bulk_rules_textarea');
        var hint = document.getElementById('bulk_rules_hint');
        if (!ta) { window.closeBulkRulesModal(); return; }

        var parts = (ta.value || '').split(/[\r\n\t;]+/);
        var seenIn = {}, parsed = [];
        for (var i = 0; i < parts.length; i++) {
            var v = String(parts[i] || '').replace(/^\s+|\s+$/g, '');
            if (!v) continue;
            if (v.charAt(0) === '#') continue;
            if (seenIn[v]) continue;
            seenIn[v] = 1;
            parsed.push(v);
        }
        if (!parsed.length) {
            if (hint) hint.textContent = 'Нет правил для добавления.';
            return;
        }

        collectFromUI();
        var g = findGroup(ctx.cat, ctx.gid);
        if (!g) { window.closeBulkRulesModal(); return; }

        g.rules = (g.rules || [])
            .map(normRuleState)
            .filter(function(x){ return x.value.trim() !== ''; });
        var existing = {};
        for (var j = 0; j < g.rules.length; j++) existing[g.rules[j].value] = 1;

        var added = 0;
        for (var k = 0; k < parsed.length; k++) {
            if (!existing[parsed[k]]) {
                g.rules.push({value: parsed[k], enabled: true});
                existing[parsed[k]] = 1;
                added++;
            }
        }
        if (added === 0) {
            if (hint) hint.textContent = 'Все правила уже есть в группе.';
            return;
        }
        if (g.collapsed) g.collapsed = false;   // авто-разворот, чтобы было видно результат
        renderAll();
        _triggerDupsScan();
        window.closeBulkRulesModal();
    };

    window.applyBulkRulesReplace = function() {
        var ctx = window.__rt_bulk_ctx;
        if (!ctx) { window.closeBulkRulesModal(); return; }
        var ta = document.getElementById('bulk_rules_textarea');
        var hint = document.getElementById('bulk_rules_hint');
        if (!ta) { window.closeBulkRulesModal(); return; }

        var parts = (ta.value || '').split(/[\r\n\t;]+/);
        var seenIn = {}, parsed = [];
        for (var i = 0; i < parts.length; i++) {
            var v = String(parts[i] || '').replace(/^\s+|\s+$/g, '');
            if (!v) continue;
            if (v.charAt(0) === '#') continue;
            if (seenIn[v]) continue;
            seenIn[v] = 1;
            parsed.push(v);
        }

        collectFromUI();
        var g = findGroup(ctx.cat, ctx.gid);
        if (!g) { window.closeBulkRulesModal(); return; }

        if (!parsed.length) {
            if (!confirm('Список пуст. Очистить все правила в этой группе?')) return;
            g.rules = [{value:'', enabled:true}];
        } else {
            g.rules = parsed.map(function(v){ return {value: v, enabled: true}; });
        }
        if (g.collapsed) g.collapsed = false;   /* развернуть, чтобы было видно результат */
        renderAll();
        _triggerDupsScan();
        window.closeBulkRulesModal();
    };

    document.addEventListener('keydown', function(e) {
        var pm = document.getElementById('paste_group_modal');
        if (pm && pm.style.display === 'flex') {
            if (e.key === 'Escape') {
                e.preventDefault();
                window.closePasteGroupModal();
                return;
            }
            if ((e.ctrlKey || e.metaKey) && (e.key === 'Enter' || e.keyCode === 13)) {
                e.preventDefault();
                window.applyPasteGroup();
                return;
            }
        }
        var m = document.getElementById('bulk_rules_modal');
        if (!m || m.style.display !== 'flex') return;
        if (e.key === 'Escape') {
            e.preventDefault();
            window.closeBulkRulesModal();
        } else if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'Enter' || e.keyCode === 13)) {
            e.preventDefault();
            window.applyBulkRulesReplace();
        } else if ((e.ctrlKey || e.metaKey) && (e.key === 'Enter' || e.keyCode === 13)) {
            e.preventDefault();
            window.applyBulkRules();
        }
    }, true);


    document.addEventListener('click', function(e) {
        var t = e.target;
        if (!t || !t.closest) return;
        var ap = t.closest('.rt-add-group-paste');
        if (ap) { onPasteGroup(ap.getAttribute('data-cat'), ap); return; }
        var ag = t.closest('.rt-add-group');
        if (ag) { onAddGroup(ag.getAttribute('data-cat')); return; }
        var ar = t.closest('.rt-add-rule');
        if (ar) {
            var card = ar.closest('.rt-group-card');
            if (card) onAddRule(card.getAttribute('data-cat'), card.getAttribute('data-gid'));
            return;
        }
        var rd = t.closest('.rt-rule-del');
        if (rd) {
            var r = rd.closest('.rt-rule');
            if (r) {
                var rulesBox = r.parentNode;
                var idx = Array.prototype.indexOf.call(rulesBox.querySelectorAll('.rt-rule'), r);
                if (idx >= 0) onDelRule(r.getAttribute('data-cat'), r.getAttribute('data-gid'), idx);
            }
            return;
        }
        var gd = t.closest('.rt-group-del');
        if (gd) {
            var gcard = gd.closest('.rt-group-card');
            if (gcard) onDelGroup(gcard.getAttribute('data-cat'), gcard.getAttribute('data-gid'));
            return;
        }
        var gt = t.closest('.rt-group-toggle');
        if (gt) {
            var gcard2 = gt.closest('.rt-group-card');
            if (gcard2) onToggleGroup(gcard2.getAttribute('data-cat'), gcard2.getAttribute('data-gid'));
            return;
        }
        var gb = t.closest('.rt-group-bulk');
        if (gb) {
            var gcard3 = gb.closest('.rt-group-card');
            if (gcard3) onBulkAddRules(gcard3.getAttribute('data-cat'), gcard3.getAttribute('data-gid'));
            return;
        }
        var gc = t.closest('.rt-group-copy');
        if (gc) {
            var gcard4 = gc.closest('.rt-group-card');
            if (gcard4) onCopyGroup(gcard4.getAttribute('data-cat'), gcard4.getAttribute('data-gid'), gc);
            return;
        }
    });

    document.addEventListener('input', function(e) {
        var t = e.target;
        if (!t || !t.matches) return;
        if (!t.closest || !t.closest('#routing_modal')) return;
        if (t.matches('textarea[name="routing_block_rules"], textarea[name="routing_direct_rules"], textarea[name="routing_proxy_rules"]')) {
            updateCatCounts();
        }
    }, true);

    document.addEventListener('change', function(e) {
        var t = e.target;
        if (!t || !t.matches) return;
        if (!t.closest || !t.closest('#routing_modal')) return;
        if (t.matches('.rt-rule-enabled')) {
            var rule = t.closest('.rt-rule');
            if (rule) rule.classList.toggle('rt-rule-off', !t.checked);
            collectFromUI();
            updateCatCounts();
            _triggerDupsScan();
        } else if (t.matches('.rt-group-enabled')) {
            var card = t.closest('.rt-group-card');
            if (card) card.classList.toggle('rt-group-off', !t.checked);
            collectFromUI();
            updateCatCounts();
            _triggerDupsScan();
        }
    }, true);

    function syncToHidden() {
        collectFromUI();
        var hid = document.getElementById('routing_groups_json');
        if (hid) hid.value = JSON.stringify(DATA);
    }

    function init() {
        var hid = document.getElementById('routing_groups_json');
        if (hid && hid.getAttribute('data-init')) {
            try {
                var d = JSON.parse(hid.getAttribute('data-init'));
                if (d && typeof d === 'object') {
                    for (var c = 0; c < CATS.length; c++) {
                        var cat = CATS[c];
                        if (Array.isArray(d[cat])) {
                            DATA[cat] = d[cat].map(function(g) {
                                if (typeof g === 'string') {
                                    g = {id: uid(), title: '', rules: [g], collapsed: false};
                                }
                                if (!g || typeof g !== 'object') g = {};
                                if (g.enabled === undefined) g.enabled = true;
                                if (!Array.isArray(g.rules)) g.rules = [];
                                g.rules = g.rules.map(normRuleState);
                                return g;
                            });
                        } else {
                            DATA[cat] = [];
                        }
                    }
                }
            } catch (e) {}
        }
        renderAll();
        var form = document.querySelector('form[action="/manage"]');
        if (form) form.addEventListener('submit', syncToHidden, true);
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else init();

    window.__rt_get_data = function() {
        collectFromUI();
        return DATA;
    };
    window.__rt_set_data = function(newData, options) {
        if (!newData || typeof newData !== 'object') return;
        for (var c = 0; c < CATS.length; c++) {
            var cat = CATS[c];
            var list = newData[cat];
            if (!Array.isArray(list)) { DATA[cat] = []; continue; }
            DATA[cat] = list.map(function(g) {
                if (typeof g === 'string') g = {id: uid(), title: '', rules: [g], collapsed: false};
                if (!g || typeof g !== 'object') g = {};
                if (!g.id) g.id = uid();
                if (g.enabled === undefined) g.enabled = true;
                if (!Array.isArray(g.rules)) g.rules = [];
                g.rules = g.rules.map(normRuleState);
                return g;
            });
        }
        renderAll();
        _triggerDupsScan(!(options && options.clean));
        var saveBtn = document.getElementById('routing_save_btn');
        if (saveBtn && !(options && options.clean)) saveBtn.classList.add('routing-dirty');
    };
})();



/* ==== Routing modal: dirty-индикатор на «Сохранить» ==== */
(function() {
    'use strict';
    if (window.__RT_STICKY_HEADER) return;
    window.__RT_STICKY_HEADER = true;

    function getBtn() {
        var b = document.getElementById('routing_save_btn');
        return b;
    }
    function markDirty() {
        var b = getBtn();
        if (!b) return;
        if (b.classList.contains('routing-dirty')) return;
        b.classList.add('routing-dirty');
    }
    function clearDirty() {
        var b = getBtn();
        if (b) b.classList.remove('routing-dirty');
    }

    function inRouting(el) {
        return !!(el && el.closest && el.closest('#routing_modal'));
    }

    document.addEventListener('input', function(e) {
        if (inRouting(e.target)) markDirty();
    }, true);
    document.addEventListener('change', function(e) {
        if (inRouting(e.target)) markDirty();
    }, true);
    document.addEventListener('click', function(e) {
        var t = e.target;
        if (!t || !t.closest || !inRouting(t)) return;
        if (t.closest('.rt-add-group') || t.closest('.rt-add-rule') ||
            t.closest('.rt-rule-del')  || t.closest('.rt-group-del') ||
            t.closest('.rt-group-toggle')) {
            markDirty();
        }
    }, true);

    document.addEventListener('submit', function(e) {
        var f = e.target;
        if (f && f.matches && f.matches('form[action="/manage"]')) clearDirty();
    }, true);

    var _origOpen = window.openRoutingModal;
    window.openRoutingModal = function() {
        clearDirty();
        var r = (typeof _origOpen === 'function')
            ? _origOpen.apply(this, arguments)
            : undefined;
        var c = document.querySelector('#routing_modal .modal-content');
        if (c) c.scrollTop = 0;
        return r;
    };
    var _origClose = window.closeRoutingModal;
    window.closeRoutingModal = function() {
        var r = (typeof _origClose === 'function')
            ? _origClose.apply(this, arguments)
            : undefined;
        clearDirty();
        return r;
    };
})();


/* ==== Routing: принудительное сворачивание групп при открытии ==== */
(function() {
    'use strict';
    if (window.__RT_AUTOCOLLAPSE) return;
    window.__RT_AUTOCOLLAPSE = true;

    function collapseAll() {
        var cards = document.querySelectorAll('#routing_modal .rt-group-card');
        for (var i = 0; i < cards.length; i++) {
            cards[i].classList.add('collapsed');
        }
    }

    /* При каждом открытии — свернуть всё */
    var _origOpen = window.openRoutingModal;
    window.openRoutingModal = function() {
        var r = (typeof _origOpen === 'function')
            ? _origOpen.apply(this, arguments)
            : undefined;
        /* даём модулю групп отрендерить DOM, потом сворачиваем */
        setTimeout(collapseAll, 0);
        return r;
    };

    /* На всякий случай — DOMContentLoaded (если модалка как-то уже открыта) */
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() {
            setTimeout(collapseAll, 100);
        });
    } else {
        setTimeout(collapseAll, 100);
    }
})();


/* ==== Routing: drag&drop групп (внутри категории) ==== */
(function() {
    'use strict';
    if (window.__RT_GROUP_DND) return;
    window.__RT_GROUP_DND = true;

    var dragCard = null;

    function clearDropHints() {
        var els = document.querySelectorAll('#routing_modal .rt-group-card.drag-over-top, #routing_modal .rt-group-card.drag-over-bottom');
        for (var i = 0; i < els.length; i++) {
            els[i].classList.remove('drag-over-top');
            els[i].classList.remove('drag-over-bottom');
        }
    }

    /* Разрешаем draggable, только когда тянут за ⠿ */
    document.addEventListener('mousedown', function(e) {
        var h = e.target.closest && e.target.closest('.rt-group-drag');
        if (!h) return;
        var card = h.closest('.rt-group-card');
        if (!card) return;
        card.setAttribute('draggable', 'true');
    }, true);

    document.addEventListener('dragstart', function(e) {
        var card = e.target.closest && e.target.closest('.rt-group-card');
        if (!card || card.getAttribute('draggable') !== 'true') return;
        dragCard = card;
        card.classList.add('dragging');
        try {
            e.dataTransfer.effectAllowed = 'move';
            e.dataTransfer.setData('text/plain', 'group');
        } catch(_) {}
    });

    document.addEventListener('dragover', function(e) {
        if (!dragCard) return;
        var target = e.target.closest && e.target.closest('.rt-group-card');
        if (!target || target === dragCard) return;
        if (target.getAttribute('data-cat') !== dragCard.getAttribute('data-cat')) return;
        var container = target.parentNode;
        if (!container || !container.classList.contains('rt-groups')) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';

        /* Подсказка сверху/снизу */
        clearDropHints();
        var rect = target.getBoundingClientRect();
        var after = e.clientY > rect.top + rect.height / 2;
        target.classList.add(after ? 'drag-over-bottom' : 'drag-over-top');

        /* Переставляем в DOM сразу же, чтобы видеть результат в реальном времени */
        container.insertBefore(dragCard, after ? target.nextSibling : target);
    });

    document.addEventListener('drop', function(e) {
        if (!dragCard) return;
        var target = e.target.closest && e.target.closest('.rt-group-card');
        if (!target) return;
        if (target.getAttribute('data-cat') !== dragCard.getAttribute('data-cat')) return;
        e.preventDefault();
    });

    function finishDrag() {
        clearDropHints();
        if (dragCard) {
            dragCard.classList.remove('dragging');
            dragCard.removeAttribute('draggable');
        }
        dragCard = null;
        /* отметить, что были изменения */
        var saveBtn = document.getElementById('routing_save_btn');
        if (saveBtn && !saveBtn.classList.contains('routing-dirty')) {
            saveBtn.classList.add('routing-dirty');
        }
    }

    document.addEventListener('dragend', function(e) {
        if (!dragCard) return;
        finishDrag();
    });

    /* На случай, если dragend не сработал (например, отпустили вне страницы) */
    document.addEventListener('mouseup', function() {
        /* убираем draggable-флаг, если drag не начался */
        var cards = document.querySelectorAll('#routing_modal .rt-group-card[draggable="true"]:not(.dragging)');
        for (var i = 0; i < cards.length; i++) cards[i].removeAttribute('draggable');
    }, true);
})();


/* ==== Routing: drag&drop правил ==== */
(function() {
    'use strict';
    if (window.__RT_RULE_DND) return;
    window.__RT_RULE_DND = true;

    var dragRule = null;

    function clearHints() {
        var els = document.querySelectorAll(
            '#routing_modal .rt-rule.drag-over-top, ' +
            '#routing_modal .rt-rule.drag-over-bottom, ' +
            '#routing_modal .rt-rules.drag-over'
        );
        for (var i = 0; i < els.length; i++) {
            els[i].classList.remove('drag-over-top');
            els[i].classList.remove('drag-over-bottom');
            els[i].classList.remove('drag-over');
        }
    }

    function markDirty() {
        var b = document.getElementById('routing_save_btn');
        if (b && !b.classList.contains('routing-dirty')) b.classList.add('routing-dirty');
    }

    /* draggable=true ставим только при mousedown по ⠿ правила */
    document.addEventListener('mousedown', function(e) {
        var h = e.target.closest && e.target.closest('.rt-rule-drag');
        if (!h) return;
        var rule = h.closest('.rt-rule');
        if (!rule) return;
        /* чтобы не конфликтовать с drag группы — ставим только правилу */
        rule.setAttribute('draggable', 'true');
    }, true);

    document.addEventListener('dragstart', function(e) {
        var rule = e.target.closest && e.target.closest('.rt-rule');
        if (!rule || rule.getAttribute('draggable') !== 'true') return;
        dragRule = rule;
        rule.classList.add('dragging');
        try {
            e.dataTransfer.effectAllowed = 'move';
            e.dataTransfer.setData('text/plain', 'rule');
        } catch(_) {}
        e.stopPropagation();
    }, true);

    document.addEventListener('dragover', function(e) {
        if (!dragRule) return;
        var cat = dragRule.getAttribute('data-cat');

        var targetRule = e.target.closest && e.target.closest('.rt-rule');
        if (targetRule && targetRule !== dragRule) {
            if (targetRule.getAttribute('data-cat') !== cat) return;
            e.preventDefault();
            if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
            clearHints();
            var rect = targetRule.getBoundingClientRect();
            var after = e.clientY > rect.top + rect.height / 2;
            targetRule.classList.add(after ? 'drag-over-bottom' : 'drag-over-top');
            var parent = targetRule.parentNode;
            if (parent && parent.classList.contains('rt-rules')) {
                parent.insertBefore(dragRule, after ? targetRule.nextSibling : targetRule);
            }
            return;
        }

        var targetContainer = e.target.closest && e.target.closest('.rt-rules');
        if (targetContainer) {
            var card = targetContainer.closest('.rt-group-card');
            if (!card) return;
            if (card.getAttribute('data-cat') !== cat) return;
            if (targetContainer === dragRule.parentNode) return;
            e.preventDefault();
            if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
            clearHints();
            targetContainer.classList.add('drag-over');
            targetContainer.appendChild(dragRule);
        }
    });

    document.addEventListener('drop', function(e) {
        if (!dragRule) return;
        var targetRule = e.target.closest && e.target.closest('.rt-rule');
        var targetContainer = e.target.closest && e.target.closest('.rt-rules');
        if (!targetRule && !targetContainer) return;
        e.preventDefault();
        e.stopPropagation();
    }, true);

    function finish() {
        clearHints();
        if (dragRule) {
            dragRule.classList.remove('dragging');
            dragRule.removeAttribute('draggable');
        }
        dragRule = null;
        markDirty();
    }

    document.addEventListener('dragend', function() {
        if (!dragRule) return;
        finish();
    });

    /* На случай, если drag не начался — снимаем флаг draggable */
    document.addEventListener('mouseup', function() {
        var rules = document.querySelectorAll('#routing_modal .rt-rule[draggable="true"]:not(.dragging)');
        for (var i = 0; i < rules.length; i++) rules[i].removeAttribute('draggable');
    }, true);
})();
/* ==== Routing: дубликаты v4 ==== */
(function() {
    'use strict';
    if (window.__RT_DUPS_V4) return;
    window.__RT_DUPS_V4 = true;

    var CATS = ['block', 'direct', 'proxy'];

    function normRule(s) {
        var v = String(s == null ? '' : s).trim().toLowerCase();
        if (!v) return '';
        v = v.replace(/\.+$/, '');
        if (v.indexOf('domain:') === 0) v = v.slice(7);
        v = v.replace(/^\s+|\s+$/g, '');
        return v;
    }

    /* Ищем cat/gid у ближайшего .rt-group-card — там реальные data-атрибуты. */
    function ctxOf(el) {
        var card = el.closest ? el.closest('.rt-group-card') : null;
        if (card) {
            return {
                cat: card.getAttribute('data-cat') || '',
                gid: card.getAttribute('data-gid') || ''
            };
        }
        var rule = el.closest ? el.closest('.rt-rule') : null;
        if (rule) {
            return {
                cat: rule.getAttribute('data-cat') || '',
                gid: rule.getAttribute('data-gid') || ''
            };
        }
        return { cat: '', gid: '' };
    }

    function scan() {
        var modal = document.getElementById('routing_modal');
        if (!modal) return;

        // --- 1. Снять старые метки ---
        var inputs = modal.querySelectorAll('.rt-rule-input');
        for (var i = 0; i < inputs.length; i++) {
            inputs[i].classList.remove('rt-dup');
            inputs[i].classList.remove('rt-dup-x');
        }
        var cards = modal.querySelectorAll('.rt-group-card');
        for (var i = 0; i < cards.length; i++) {
            var c = cards[i];
            c.removeAttribute('data-has-dup');
            c.removeAttribute('data-has-conf');
            var old = c.querySelector('.rt-dup-badge-group');
            if (old && old.parentNode) old.parentNode.removeChild(old);
        }
        var catBadges = modal.querySelectorAll('.rt-dup-badge');
        for (var i = 0; i < catBadges.length; i++) {
            if (catBadges[i].parentNode) catBadges[i].parentNode.removeChild(catBadges[i]);
        }

        // --- 2. Собираем карту: нормализованное правило -> [input, ...] ---
        var map = {};
        for (var i = 0; i < inputs.length; i++) {
            var el = inputs[i];
            var rEl = el.closest('.rt-rule');
            if (rEl && rEl.classList.contains('rt-rule-off')) continue;
            var cEl = el.closest('.rt-group-card');
            if (cEl && cEl.classList.contains('rt-group-off')) continue;
            var chkEl = rEl ? rEl.querySelector('.rt-rule-enabled') : null;
            if (chkEl && !chkEl.checked) continue;
            var v = normRule(el.value);
            if (!v) continue;
            if (!map[v]) map[v] = [];
            map[v].push(el);
        }

        var totalDup = 0, totalConf = 0;
        var perCat = { block: { d: 0, x: 0 }, direct: { d: 0, x: 0 }, proxy: { d: 0, x: 0 } };
        var perGroup = {};

        Object.keys(map).forEach(function(k) {
            var arr = map[k];
            if (arr.length < 2) return;

            var byCat = {};
            for (var i = 0; i < arr.length; i++) {
                var cx = ctxOf(arr[i]);
                byCat[cx.cat] = (byCat[cx.cat] || 0) + 1;
            }
            var hasSameCat = false;
            Object.keys(byCat).forEach(function(c) {
                if (byCat[c] > 1) hasSameCat = true;
            });

            if (hasSameCat) totalDup += arr.length;
            else            totalConf += arr.length;

            for (var i = 0; i < arr.length; i++) {
                var el2 = arr[i];
                var cx2 = ctxOf(el2);
                el2.classList.add(hasSameCat ? 'rt-dup' : 'rt-dup-x');

                if (!perCat[cx2.cat]) perCat[cx2.cat] = { d: 0, x: 0 };
                if (hasSameCat) perCat[cx2.cat].d++;
                else            perCat[cx2.cat].x++;

                var gkey = cx2.cat + '::' + cx2.gid;
                if (!perGroup[gkey]) perGroup[gkey] = { d: 0, x: 0 };
                if (hasSameCat) perGroup[gkey].d++;
                else            perGroup[gkey].x++;
            }
        });

        // --- 3. Бейдж у заголовка категории ---
        CATS.forEach(function(cat) {
            var pc = perCat[cat];
            if (!pc || (pc.d === 0 && pc.x === 0)) return;
            var box = document.getElementById('rt_groups_' + cat);
            if (!box) return;
            var field = box.closest('.setting-field');
            if (!field) return;
            var label = field.querySelector('.setting-label');
            if (!label) return;
            var text = '';
            if (pc.d > 0) text += '\u26A0 ' + pc.d + ' \u0434\u0443\u0431\u043B.';
            if (pc.x > 0) text += (text ? ' ' : '') + '\u26A0 ' + pc.x + ' \u043A\u043E\u043D\u0444\u043B.';
            var span = document.createElement('span');
            span.className = 'rt-dup-badge' + (pc.d === 0 ? ' rt-dup-badge-x' : '');
            span.textContent = text;
            label.appendChild(span);
        });

        // --- 4. Бейдж у шапки каждой группы ---
        for (var i = 0; i < cards.length; i++) {
            var card = cards[i];
            var cat = card.getAttribute('data-cat') || '';
            var gid = card.getAttribute('data-gid') || '';
            var key = cat + '::' + gid;
            var gi = perGroup[key];
            if (!gi || (gi.d === 0 && gi.x === 0)) continue;

            if (gi.d > 0) card.setAttribute('data-has-dup', '1');
            else          card.setAttribute('data-has-conf', '1');

            var head = card.querySelector('.rt-group-head');
            if (!head) continue;
            if (head.querySelector('.rt-dup-badge-group')) continue;

            var text2 = '';
            if (gi.d > 0) text2 += '\u26A0 ' + gi.d;
            if (gi.x > 0) text2 += (text2 ? ' / ' : '') + '\u26A0 ' + gi.x + ' \u043A\u043E\u043D\u0444\u043B.';
            var b = document.createElement('span');
            b.className = 'rt-dup-badge-group' + (gi.d === 0 ? ' rt-dup-badge-group-x' : '');
            b.textContent = text2;
            b.title = (gi.d > 0 ? gi.d + ' \u0434\u0443\u0431\u043B\u0438\u043A\u0430\u0442(\u043E\u0432) \u0432 \u0433\u0440\u0443\u043F\u043F\u0435' : '') +
                      (gi.x > 0 ? (gi.d ? '; ' : '') + gi.x + ' \u043A\u043E\u043D\u0444\u043B\u0438\u043A\u0442(\u043E\u0432) \u0441 \u0434\u0440\u0443\u0433\u043E\u0439 \u043A\u0430\u0442\u0435\u0433\u043E\u0440\u0438\u0435\u0439' : '');
            var titleEl = head.querySelector('.rt-group-title');
            if (titleEl && titleEl.nextSibling) head.insertBefore(b, titleEl.nextSibling);
            else head.appendChild(b);
        }

        // --- 5. Кнопка «Сохранить» ---
        var btn = document.getElementById('routing_save_btn');
        if (btn) {
            if (totalDup > 0 || totalConf > 0) {
                btn.classList.add('routing-has-dups', 'routing-blocked');
            } else {
                btn.classList.remove('routing-has-dups', 'routing-blocked');
            }
        }
    }

    function schedule() {
        if (schedule._t) return;
        schedule._t = setTimeout(function() {
            schedule._t = 0;
            scan();
        }, 60);
    }

    document.addEventListener('input', function(e) {
        var t = e.target;
        if (t && t.classList && t.classList.contains('rt-rule-input') &&
            t.closest && t.closest('#routing_modal')) schedule();
    }, true);

    document.addEventListener('click', function(e) {
        var t = e.target;
        if (!t || !t.closest || !t.closest('#routing_modal')) return;
        if (t.closest('.rt-add-rule') || t.closest('.rt-add-group') ||
            t.closest('.rt-rule-del') || t.closest('.rt-group-del') ||
            t.closest('.rt-group-toggle')) {
            schedule();
        }
    }, true);

    document.addEventListener('dragend', function(e) {
        if (!e.target || !e.target.closest || !e.target.closest('#routing_modal')) return;
        schedule();
    }, true);

    document.addEventListener('submit', function(e) {
        var f = e.target;
        if (!f || !f.matches || !f.matches('form[action="/manage"]')) return;
        var sub = e.submitter || document.activeElement;
        if (!sub || sub.id !== 'routing_save_btn') return;
        scan();
        var btn = document.getElementById('routing_save_btn');
        if (btn && btn.classList.contains('routing-blocked')) {
            e.preventDefault();
            e.stopImmediatePropagation();
            window.alert('⚠ Нельзя сохранить:\n\nВ правилах есть дубликаты или конфликты.\nУберите подсвеченные строки или исправьте их.');
        }
    }, true);

    function installObserver() {
        var modal = document.getElementById('routing_modal');
        if (!modal) { setTimeout(installObserver, 300); return; }
        if (!window.MutationObserver) return;
        try {
            var mo = new MutationObserver(function(muts) {
                for (var i = 0; i < muts.length; i++) {
                    var m = muts[i];
                    var changed = Array.from(m.addedNodes || []).concat(Array.from(m.removedNodes || []));
                    for (var a = 0; a < changed.length; a++) {
                        var n = changed[a];
                        if (n.nodeType !== 1) continue;
                        if (n.classList && (
                            n.classList.contains('rt-group-card') ||
                            n.classList.contains('rt-rule') ||
                            n.classList.contains('rt-rule-input') ||
                            n.classList.contains('rt-groups')) ||
                            n.querySelector('.rt-group-card,.rt-rule,.rt-rule-input,.rt-groups')) {
                            schedule(); return;
                        }
                    }
                }
            });
            mo.observe(modal, { childList: true, subtree: true });
        } catch (e) {}
    }
    installObserver();

    var _origOpen = window.openRoutingModal;
    window.openRoutingModal = function() {
        var r = (typeof _origOpen === 'function')
            ? _origOpen.apply(this, arguments)
            : undefined;
        setTimeout(scan, 80);
        return r;
    };

    setInterval(function() {
        var modal = document.getElementById('routing_modal');
        if (modal && modal.style.display === 'flex') scan();
    }, 1500);

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() { setTimeout(scan, 200); });
    } else {
        setTimeout(scan, 200);
    }
})();



/* ==== OptiRout-style interface picker: sync display on change ==== */
(function() {
    'use strict';
    if (window.__MT_IFACE_PICKER_JS) return;
    window.__MT_IFACE_PICKER_JS = true;

    function _mtSyncPicker(sel) {
        var picker = sel.closest && sel.closest('.mt-iface-picker');
        if (!picker) return;
        var opt = sel.options && sel.options[sel.selectedIndex];
        var nameEl = picker.querySelector('.mt-iface-name');
        var descEl = picker.querySelector('.mt-iface-desc');
        if (!nameEl) return;
        var name = '';
        var desc = '';
        if (opt) {
            name = opt.getAttribute('data-name') || opt.value || '';
            desc = opt.getAttribute('data-desc') || '';
        }
        nameEl.textContent = name || 'Интерфейс';
        if (descEl) {
            if (desc) {
                descEl.textContent = desc;
                descEl.style.display = '';
            } else {
                descEl.textContent = '';
                descEl.style.display = 'none';
            }
        }
    }

    document.addEventListener('change', function(e) {
        var t = e.target;
        if (!t || !t.matches || !t.matches('.mt-iface-picker .mt-group-iface')) return;
        _mtSyncPicker(t);
    }, true);

    window.__mtSyncIfacePickers = function() {
        var sels = document.querySelectorAll('.mt-iface-picker .mt-group-iface');
        for (var i = 0; i < sels.length; i++) _mtSyncPicker(sels[i]);
    };

    if (window.console && console.log) {
        console.log('optiray: mt-iface-picker installed');
    }
})();

/* ==== OptiRout: подсветка дубликатов (по всем группам) ==== */
(function() {
    'use strict';
    if (window.__MT_DUPS) return;
    window.__MT_DUPS = true;

    function normRule(type, val) {
        var v = String(val == null ? '' : val).trim().toLowerCase();
        if (!v) return '';
        var t = String(type || 'namespace').toLowerCase();
        v = v.replace(/\.+$/, '');
        return t + ':' + v;
    }

    function scan() {
        var modal = document.getElementById('mt_modal');
        if (!modal || modal.style.display !== 'flex') return;

        // 1. Снять старые метки
        var inputs = modal.querySelectorAll('.mt-rule-input');
        for (var i = 0; i < inputs.length; i++) {
            inputs[i].classList.remove('mt-dup');
            inputs[i].classList.remove('mt-dup-x');
        }
        var cards = modal.querySelectorAll('.mt-group-card');
        for (var i = 0; i < cards.length; i++) {
            cards[i].removeAttribute('data-has-dup');
            cards[i].removeAttribute('data-has-conf');
            var old = cards[i].querySelector('.mt-dup-badge-group');
            if (old && old.parentNode) old.parentNode.removeChild(old);
        }

        // 2. Собрать карту: normalized_rule -> [{el, gid}]
        var map = {};
        for (var i = 0; i < inputs.length; i++) {
            var el = inputs[i];
            var rEl = el.closest('.mt-rule');
            if (!rEl) continue;
            var chk = rEl.querySelector('.mt-rule-enable');
            if (chk && !chk.checked) continue;
            var card = el.closest('.mt-group-card');
            if (card && card.classList.contains('mt-group-off')) continue;
            var typeEl = rEl.querySelector('.mt-rule-type');
            var type = typeEl ? typeEl.value : 'namespace';
            var v = normRule(type, el.value);
            if (!v) continue;
            if (!map[v]) map[v] = [];
            map[v].push({
                el: el,
                gid: card ? (card.getAttribute('data-gid') || '') : ''
            });
        }

        // 3. Разметить дубликаты
        var perGroup = {};
        var totalDup = 0, totalConf = 0;

        Object.keys(map).forEach(function(k) {
            var arr = map[k];
            if (arr.length < 2) return;
            var byGid = {};
            for (var i = 0; i < arr.length; i++) {
                var g = arr[i].gid;
                byGid[g] = (byGid[g] || 0) + 1;
            }
            var hasSameGroup = false;
            Object.keys(byGid).forEach(function(g) { if (byGid[g] > 1) hasSameGroup = true; });

            if (hasSameGroup) totalDup += arr.length;
            else              totalConf += arr.length;

            for (var i = 0; i < arr.length; i++) {
                var a = arr[i];
                a.el.classList.add(hasSameGroup ? 'mt-dup' : 'mt-dup-x');
                if (!perGroup[a.gid]) perGroup[a.gid] = { d: 0, x: 0 };
                if (hasSameGroup) perGroup[a.gid].d++;
                else              perGroup[a.gid].x++;
            }
        });

        // 4. Бейджи на группах
        for (var i = 0; i < cards.length; i++) {
            var card2 = cards[i];
            var gid2 = card2.getAttribute('data-gid') || '';
            var gi = perGroup[gid2];
            if (!gi || (gi.d === 0 && gi.x === 0)) continue;

            if (gi.d > 0) card2.setAttribute('data-has-dup', '1');
            else          card2.setAttribute('data-has-conf', '1');

            var head = card2.querySelector('.mt-group-head');
            if (!head) continue;
            if (head.querySelector('.mt-dup-badge-group')) continue;

            var text = '';
            if (gi.d > 0) text += '\u26A0 ' + gi.d;
            if (gi.x > 0) text += (text ? ' / ' : '') + '\u26A0 ' + gi.x + ' \u043A\u043E\u043D\u0444\u043B.';
            var b = document.createElement('span');
            b.className = 'mt-dup-badge-group' + (gi.d === 0 ? ' mt-dup-badge-group-x' : '');
            b.textContent = text;
            b.title = (gi.d > 0 ? gi.d + ' \u0434\u0443\u0431\u043B\u0438\u043A\u0430\u0442(\u043E\u0432) \u0432 \u0433\u0440\u0443\u043F\u043F\u0435' : '') +
                      (gi.x > 0 ? (gi.d ? '; ' : '') + gi.x + ' \u043A\u043E\u043D\u0444\u043B\u0438\u043A\u0442(\u043E\u0432) \u0441 \u0434\u0440\u0443\u0433\u043E\u0439 \u0433\u0440\u0443\u043F\u043F\u043E\u0439' : '');
            var titleEl = head.querySelector('.mt-group-name');
            if (titleEl && titleEl.nextSibling) head.insertBefore(b, titleEl.nextSibling);
            else head.appendChild(b);
        }

        // 5. Индикатор на кнопке Сохранить
        var btn = document.getElementById('mt_save_btn');
        if (btn) {
            var hasIssues = (totalDup > 0 || totalConf > 0);
            btn.classList.toggle('mt-has-dups', hasIssues);
            btn.title = hasIssues
                ? ('\u26A0 \u0414\u0443\u0431\u043B\u0438\u043A\u0430\u0442\u044B: '
                   + totalDup + ' \u0432\u043D\u0443\u0442\u0440\u0438 \u0433\u0440\u0443\u043F\u043F, '
                   + totalConf + ' \u043C\u0435\u0436\u0434\u0443 \u0433\u0440\u0443\u043F\u043F\u0430\u043C\u0438')
                : '';
        }

        if (window.console && (totalDup || totalConf)) {
            console.log('optiray: MT \u0434\u0443\u0431\u043B\u0438\u043A\u0430\u0442\u044B: ' + totalDup
                        + ', \u043A\u043E\u043D\u0444\u043B\u0438\u043A\u0442\u044B: ' + totalConf);
        }
    }

    function schedule() {
        if (schedule._t) return;
        schedule._t = setTimeout(function() {
            schedule._t = 0;
            scan();
        }, 80);
    }

    document.addEventListener('input', function(e) {
        var t = e.target;
        if (t && t.classList && t.classList.contains('mt-rule-input') &&
            t.closest && t.closest('#mt_modal')) schedule();
    }, true);

    document.addEventListener('change', function(e) {
        var t = e.target;
        if (!t || !t.closest || !t.closest('#mt_modal')) return;
        if (t.classList && (t.classList.contains('mt-rule-type') ||
                            t.classList.contains('mt-rule-enable') ||
                            t.classList.contains('mt-group-enable'))) {
            schedule();
        }
    }, true);

    document.addEventListener('click', function(e) {
        var t = e.target;
        if (!t || !t.closest || !t.closest('#mt_modal')) return;
        if (t.closest('.mt-add-rule') || t.closest('.rt-rule-del') ||
            t.closest('.mt-group-card .rt-group-del') ||
            t.closest('.mt-group-toggle') || t.closest('.mt-add-group')) {
            schedule();
        }
    }, true);

    document.addEventListener('dragend', function(e) {
        if (!e.target || !e.target.closest || !e.target.closest('#mt_modal')) return;
        schedule();
    }, true);

    function installObserver() {
        var modal = document.getElementById('mt_modal');
        if (!modal) { setTimeout(installObserver, 300); return; }
        if (!window.MutationObserver) return;
        try {
            var mo = new MutationObserver(function(muts) {
                for (var i = 0; i < muts.length; i++) {
                    var added = muts[i].addedNodes || [];
                    for (var j = 0; j < added.length; j++) {
                        var n = added[j];
                        if (n.nodeType !== 1) continue;
                        if (n.classList && (n.classList.contains('mt-group-card') ||
                                            n.classList.contains('mt-rule') ||
                                            n.classList.contains('mt-rule-input') ||
                                            n.classList.contains('mt-rules'))) {
                            schedule();
                            return;
                        }
                    }
                }
            });
            mo.observe(modal, {childList: true, subtree: true});
        } catch(e) {}
    }
    installObserver();

    var _origOpen = window.openMtModal;
    window.openMtModal = function() {
        var r = (typeof _origOpen === 'function')
            ? _origOpen.apply(this, arguments)
            : undefined;
        setTimeout(scan, 150);
        return r;
    };

    setInterval(function() {
        var modal = document.getElementById('mt_modal');
        if (modal && modal.style.display === 'flex') scan();
    }, 2000);

    // Экспорт для внешних проверок (save guard)
    window.__mtScanDups = scan;

    if (window.console && console.log) {
        console.log('optiray: MT duplicate highlighter installed');
    }
})();

/* ==== OptiRout: запрет сохранения при дублях ==== */
(function() {
    'use strict';
    if (window.__MT_SAVE_GUARD) return;
    window.__MT_SAVE_GUARD = true;

    function _mtCountDups() {
        var modal = document.getElementById('mt_modal');
        if (!modal || modal.style.display !== 'flex') return { dups: 0, confs: 0 };

        // Форсируем свежий пересчёт, чтобы не было race с debounce
        if (typeof window.__mtScanDups === 'function') {
            try { window.__mtScanDups(); } catch(e) {}
        }
        return {
            dups:  modal.querySelectorAll('.mt-rule-input.mt-dup').length,
            confs: modal.querySelectorAll('.mt-rule-input.mt-dup-x').length
        };
    }

    function _mtShowBlockAlert(dups, confs) {
        var parts = [];
        if (dups > 0)  parts.push(dups  + ' \u0434\u0443\u0431\u043B\u0438\u043A\u0430\u0442(\u043E\u0432) \u0432\u043D\u0443\u0442\u0440\u0438 \u0433\u0440\u0443\u043F\u043F');
        if (confs > 0) parts.push(confs + ' \u043A\u043E\u043D\u0444\u043B\u0438\u043A\u0442(\u043E\u0432) \u043C\u0435\u0436\u0434\u0443 \u0433\u0440\u0443\u043F\u043F\u0430\u043C\u0438');
        var msg =
            '\u26A0 \u041D\u0435\u043B\u044C\u0437\u044F \u0441\u043E\u0445\u0440\u0430\u043D\u0438\u0442\u044C \u043A\u043E\u043D\u0444\u0438\u0433\n\n' +
            '\u041E\u0431\u043D\u0430\u0440\u0443\u0436\u0435\u043D\u043E: ' + parts.join(', ') + '.\n\n' +
            '\u041F\u043E\u0434\u0441\u0432\u0435\u0447\u0435\u043D\u043D\u044B\u0435 \u0441\u0442\u0440\u043E\u043A\u0438 ' +
            '(\u043A\u0440\u0430\u0441\u043D\u044B\u043C \u2014 \u0434\u0443\u0431\u043B\u0438\u043A\u0430\u0442\u044B, \u043E\u0440\u0430\u043D\u0436\u0435\u0432\u044B\u043C \u2014 \u043A\u043E\u043D\u0444\u043B\u0438\u043A\u0442\u044B) ' +
            '\u043D\u0443\u0436\u043D\u043E \u0443\u0434\u0430\u043B\u0438\u0442\u044C, \u0438\u0437\u043C\u0435\u043D\u0438\u0442\u044C \u0438\u043B\u0438 \u043E\u0442\u043A\u043B\u044E\u0447\u0438\u0442\u044C.';
        alert(msg);
    }

    // Обновить визуальное состояние кнопки «Сохранить»
    function _mtRefreshSaveBtn() {
        var btn = document.getElementById('mt_save_btn');
        if (!btn) return;
        var dialog = document.getElementById('mt_modal');
        if (dialog && dialog.dataset.ruleEngine === 'xray') {
            btn.classList.remove('mt-blocked', 'mt-has-dups');
            btn.removeAttribute('aria-disabled');
            btn.title = '';
            return;
        }
        var r = _mtCountDups();
        var blocked = (r.dups > 0 || r.confs > 0);

        btn.classList.toggle('mt-blocked', blocked);
        btn.classList.toggle('mt-has-dups', blocked);

        if (blocked) {
            var parts = [];
            if (r.dups > 0)  parts.push(r.dups  + ' \u0434\u0443\u0431\u043B.');
            if (r.confs > 0) parts.push(r.confs + ' \u043A\u043E\u043D\u0444\u043B.');
            btn.title = '\u26A0 \u041D\u0435\u043B\u044C\u0437\u044F \u0441\u043E\u0445\u0440\u0430\u043D\u0438\u0442\u044C: ' + parts.join(', ');
            btn.setAttribute('aria-disabled', 'true');
        } else {
            btn.title = '';
            btn.removeAttribute('aria-disabled');
        }
    }
    window.__mtRefreshSaveBtn = _mtRefreshSaveBtn;

    // Основная защита: перехват saveMtConfig
    function _mtInstallGuard() {
        if (typeof window.saveMtConfig !== 'function') {
            // Функция ещё не определена — подождём
            setTimeout(_mtInstallGuard, 200);
            return;
        }
        if (window.saveMtConfig.__mtGuarded) return;

        var _orig = window.saveMtConfig;
        var guarded = function() {
            var r = _mtCountDups();
            if (r.dups > 0 || r.confs > 0) {
                _mtShowBlockAlert(r.dups, r.confs);
                _mtRefreshSaveBtn();
                return;
            }
            return _orig.apply(this, arguments);
        };
        guarded.__mtGuarded = true;
        window.saveMtConfig = guarded;
    }
    _mtInstallGuard();

    // Резервная защита: перехват submit формы (на случай нестандартного пути)
    document.addEventListener('submit', function(e) {
        var f = e.target;
        if (!f || !f.closest || !f.closest('#mt_modal')) return;
        var r = _mtCountDups();
        if (r.dups > 0 || r.confs > 0) {
            e.preventDefault();
            e.stopImmediatePropagation();
            _mtShowBlockAlert(r.dups, r.confs);
        }
    }, true);

    // Live-обновление кнопки: слушаем изменения в модалке
    document.addEventListener('input', function(e) {
        if (!e.target || !e.target.closest || !e.target.closest('#mt_modal')) return;
        setTimeout(_mtRefreshSaveBtn, 100);
    }, true);

    document.addEventListener('change', function(e) {
        if (!e.target || !e.target.closest || !e.target.closest('#mt_modal')) return;
        setTimeout(_mtRefreshSaveBtn, 100);
    }, true);

    document.addEventListener('click', function(e) {
        var t = e.target;
        if (!t || !t.closest || !t.closest('#mt_modal')) return;
        if (t.closest('.mt-add-rule') || t.closest('.rt-rule-del') ||
            t.closest('.mt-group-card .rt-group-del') ||
            t.closest('.mt-group-toggle') || t.closest('.mt-add-group')) {
            setTimeout(_mtRefreshSaveBtn, 120);
        }
    }, true);

    // Пересчёт при открытии модалки
    var _origOpen = window.openMtModal;
    window.openMtModal = function() {
        var r = (typeof _origOpen === 'function')
            ? _origOpen.apply(this, arguments)
            : undefined;
        setTimeout(_mtRefreshSaveBtn, 250);
        return r;
    };

    // Периодический пересчёт (страховка от пропущенных событий)
    setInterval(function() {
        var modal = document.getElementById('mt_modal');
        if (modal && modal.style.display === 'flex') _mtRefreshSaveBtn();
    }, 1500);

    if (window.console && console.log) {
        console.log('optiray: MT save guard installed');
    }
})();

/* ==== OptiRout: копирование группы в буфер ==== */
(function() {
    'use strict';
    if (window.__MT_COPY_GROUP) return;
    window.__MT_COPY_GROUP = true;

    function _mtCopyText(text) {
        if (navigator.clipboard && window.isSecureContext) {
            return navigator.clipboard.writeText(text);
        }
        return new Promise(function(resolve, reject) {
            try {
                var ta = document.createElement('textarea');
                ta.value = text;
                ta.setAttribute('readonly', '');
                ta.style.position = 'fixed';
                ta.style.top = '-1000px';
                ta.style.left = '-1000px';
                document.body.appendChild(ta);
                ta.select();
                ta.setSelectionRange(0, ta.value.length);
                var ok = document.execCommand('copy');
                document.body.removeChild(ta);
                ok ? resolve() : reject(new Error('copy failed'));
            } catch (e) { reject(e); }
        });
    }

    function _mtFlashCopyBtn(btn, ok) {
        if (!btn) return;
        var orig = btn.innerHTML;
        var svgOk  = '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="20 6 9 17 4 12"/></svg>';
        var svgErr = '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>';
        btn.innerHTML = ok ? svgOk : svgErr;
        btn.classList.add(ok ? 'mt-copy-ok' : 'mt-copy-err');
        setTimeout(function() {
            btn.innerHTML = orig;
            btn.classList.remove('mt-copy-ok', 'mt-copy-err');
        }, 900);
    }

    function _mtFindGroup(gid) {
        var cfg = MT_STATE && MT_STATE.config;
        if (!cfg) return null;
        var list = cfg.groups || [];
        for (var i = 0; i < list.length; i++) {
            if (list[i] && list[i].id === gid) return list[i];
        }
        return null;
    }

    function _mtCopyGroup(btn, gid) {
        /* Перед копированием — забрать актуальные значения из DOM,
           чтобы пользователь получил в буфере то, что видит. */
        if (typeof _mtCollectFromUi === 'function') {
            _mtCollectFromUi(true);
        }
        var g = _mtFindGroup(gid);
        if (!g) { _mtFlashCopyBtn(btn, false); return; }

        var lines = [];
        var name = String(g.name || '').trim();
        lines.push(name || 'Группа');

        var rs = g.rules || [];
        for (var i = 0; i < rs.length; i++) {
            var v = String((rs[i] && rs[i].rule) || '').trim();
            if (v) lines.push(v);
        }

        var text = lines.join('\n');
        _mtCopyText(text).then(function() {
            _mtFlashCopyBtn(btn, true);
            if (window.console) {
                console.log('optiray: скопирована группа:\n' + text);
            }
        }).catch(function(e) {
            _mtFlashCopyBtn(btn, false);
            if (window.console) console.warn('optiray: copy failed:', e);
            alert('Не удалось скопировать в буфер обмена.\n\n' + (e && e.message ? e.message : e));
        });
    }

    document.addEventListener('click', function(e) {
        var t = e.target;
        if (!t || !t.closest) return;
        var cp = t.closest('.mt-group-copy');
        if (!cp) return;
        var card = cp.closest('.mt-group-card');
        if (!card) return;
        var gid = card.getAttribute('data-gid');
        if (!gid) return;
        e.stopPropagation();
        _mtCopyGroup(cp, gid);
    }, true);

    if (window.console && console.log) {
        console.log('optiray: MT group copy installed');
    }
})();

/* ==== OptiRout: автораспознавание типа правила ==== */
(function() {
    'use strict';
    if (window.__MT_TYPE_AUTODETECT) return;
    window.__MT_TYPE_AUTODETECT = true;

    var LABELS = {
        namespace: 'namespace (домен: youtube.com)',
        wildcard:  'wildcard (*.youtube.com или *)',
        subnet:    'subnet (IP / CIDR / диапазон)'
    };

    function mtDetectRuleType(val) {
        var v = String(val || '').trim();
        if (!v) return null;

        // Wildcard: *.example.com или одиночная *
        if (v.indexOf('*') >= 0) return 'wildcard';

        // IPv4 (с необязательным CIDR)
        if (/^\d{1,3}(\.\d{1,3}){3}(\/\d{1,2})?$/.test(v)) return 'subnet';

        // Диапазон IPv4: 1.2.3.4-1.2.3.10
        if (/^\d{1,3}(\.\d{1,3}){3}-\d{1,3}(\.\d{1,3}){3}$/.test(v)) return 'subnet';

        // IPv6 (с необязательным CIDR)
        if (v.indexOf(':') >= 0 && /^[0-9a-fA-F:]+(\/\d{1,3})?$/.test(v)) return 'subnet';

        // Домен: минимум одна точка, буквы/цифры/дефис
        if (/^[a-zA-Z0-9]([a-zA-Z0-9\-_]{0,62}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-_]{0,62}[a-zA-Z0-9])?)+$/.test(v)) {
            return 'namespace';
        }

        // Одиночное слово без точки — тоже namespace (OptiRout это допускает)
        if (/^[a-zA-Z0-9]([a-zA-Z0-9\-_]{0,62}[a-zA-Z0-9])?$/.test(v)) return 'namespace';

        return null;
    }

    function mtApplyDetection(ruleEl, autoSwitch) {
        var inputEl = ruleEl.querySelector('.mt-rule-input');
        var typeEl = ruleEl.querySelector('.mt-rule-type');
        if (!inputEl || !typeEl) return;

        var raw = inputEl.value;
        var detected = mtDetectRuleType(raw);
        var manual = typeEl.getAttribute('data-manual') === '1';

        // Пусто → снять ручной флаг и подсветку
        if (!String(raw).trim()) {
            typeEl.removeAttribute('data-manual');
            typeEl.classList.remove('mt-type-mismatch');
            typeEl.removeAttribute('title');
            return;
        }

        // Автопереключение (только при вводе и без ручного выбора)
        if (autoSwitch && !manual && detected && detected !== typeEl.value) {
            typeEl.value = detected;
            // Вспышка — пользователь видит, что тип переключился
            typeEl.classList.remove('mt-type-flash');
            void typeEl.offsetWidth;
            typeEl.classList.add('mt-type-flash');
            setTimeout(function() { typeEl.classList.remove('mt-type-flash'); }, 900);
            if (typeof window.__mtScanDups === 'function') {
                setTimeout(window.__mtScanDups, 60);
            }
        }

        // Подсветка расхождения
        if (detected && detected !== typeEl.value) {
            typeEl.classList.add('mt-type-mismatch');
            typeEl.title = 'Похоже на тип «' + (LABELS[detected] || detected) + '»';
        } else {
            typeEl.classList.remove('mt-type-mismatch');
            typeEl.removeAttribute('title');
        }
    }

    // Ввод в поле правила — автопереключение разрешено
    document.addEventListener('input', function(e) {
        var t = e.target;
        if (!t || !t.classList || !t.classList.contains('mt-rule-input')) return;
        if (!t.closest || !t.closest('#mt_modal')) return;
        var ruleEl = t.closest('.mt-rule');
        if (!ruleEl) return;
        mtApplyDetection(ruleEl, true);
    }, true);

    // Пользователь вручную меняет тип — ставим флаг manual
    document.addEventListener('change', function(e) {
        var t = e.target;
        if (!t || !t.classList || !t.classList.contains('mt-rule-type')) return;
        if (!t.closest || !t.closest('#mt_modal')) return;
        var ruleEl = t.closest('.mt-rule');
        if (!ruleEl) return;
        t.setAttribute('data-manual', '1');
        mtApplyDetection(ruleEl, false);
    }, true);

    // При открытии/перерисовке — только подсветка, без автопереключения
    function scanAll() {
        var modal = document.getElementById('mt_modal');
        if (!modal || modal.style.display !== 'flex') return;
        var rules = modal.querySelectorAll('.mt-rule');
        for (var i = 0; i < rules.length; i++) {
            mtApplyDetection(rules[i], false);
        }
    }
    window.__mtScanRuleTypes = scanAll;

    var _origOpen = window.openMtModal;
    window.openMtModal = function() {
        var r = (typeof _origOpen === 'function')
            ? _origOpen.apply(this, arguments)
            : undefined;
        setTimeout(scanAll, 250);
        return r;
    };

    setInterval(function() {
        var modal = document.getElementById('mt_modal');
        if (modal && modal.style.display === 'flex') scanAll();
    }, 2000);

    window.mtDetectRuleType = mtDetectRuleType;
    window.mtParseImportedRule = function(value) {
        var rule = String(value || '').trim();
        var explicit = /^(namespace|wildcard|subnet)\s*:\s*(.*)$/i.exec(rule);
        return { rule: explicit ? explicit[2].trim() : rule,
            type: explicit ? explicit[1].toLowerCase() : (mtDetectRuleType(rule) || 'namespace') };
    };
    if (window.console && console.log) console.log('optiray: MT rule type autodetect installed');
})();


/* ==== OptiRout: добавление правил списком ==== */
(function() {
    'use strict';
    if (window.__MT_BULK_RULES) return;
    window.__MT_BULK_RULES = true;

    function _mtSplitRules(raw) {
        var parts = String(raw == null ? '' : raw).split(/[\r\n\t;]+/);
        var out = [], seen = {};
        for (var i = 0; i < parts.length; i++) {
            var v = String(parts[i] || '').replace(/^\s+|\s+$/g, '');
            if (!v) continue;
            if (v.charAt(0) === '#') continue;
            if (seen[v]) continue;
            seen[v] = 1;
            out.push(v);
        }
        return out;
    }

    function _mtFindGroupById(gid) {
        var cfg = MT_STATE && MT_STATE.config;
        if (!cfg) return null;
        var list = cfg.groups || [];
        for (var i = 0; i < list.length; i++) {
            if (list[i] && list[i].id === gid) return list[i];
        }
        return null;
    }

    function _mtHint(text) {
        var h = document.getElementById('mt_bulk_rules_hint');
        if (h) h.textContent = text || '';
    }

    window.mtOpenBulkRules = function(gid) {
        if (!gid) return;
        var m = document.getElementById('mt_bulk_rules_modal');
        var ta = document.getElementById('mt_bulk_rules_textarea');
        if (!m || !ta) { alert('Модалка не найдена'); return; }
        ta.value = '';
        _mtHint('');
        window.__mt_bulk_ctx = { gid: gid };
        m.style.display = 'flex';
        setTimeout(function() { try { ta.focus(); } catch(e) {} }, 60);
    };

    window.closeMtBulkRulesModal = function() {
        var m = document.getElementById('mt_bulk_rules_modal');
        if (m) m.style.display = 'none';
        window.__mt_bulk_ctx = null;
        _mtHint('');
    };

    function _mtUncollapseGroup(gid) {
        if (!MT_UI || !MT_UI.collapsed) return;
        var card = document.querySelector('#mt_groups_container .mt-group-card[data-gid="' + gid + '"]');
        if (!card) return;
        if (card.classList.contains('collapsed')) {
            MT_UI.collapsed[gid] = false;
            /* Снимаем collapsed с DOM напрямую — быстрее, чем полный rerender */
            card.classList.remove('collapsed');
            var tgl = card.querySelector('.mt-group-toggle');
            if (tgl) tgl.setAttribute('title', 'Свернуть');
        }
    }

    window.mtApplyBulkRules = function() {
        if (document.getElementById('mt_bulk_rules_modal_replace').checked) return window.mtApplyBulkRulesReplace();
        var ctx = window.__mt_bulk_ctx;
        if (!ctx) { window.closeMtBulkRulesModal(); return; }
        var ta = document.getElementById('mt_bulk_rules_textarea');
        if (!ta) { window.closeMtBulkRulesModal(); return; }
        var parsed = _mtSplitRules(ta.value);
        if (!parsed.length) { _mtHint('Нет правил для добавления.'); return; }

        if (typeof _mtCollectFromUi === 'function') _mtCollectFromUi(true);
        var g = _mtFindGroupById(ctx.gid);
        if (!g) { window.closeMtBulkRulesModal(); return; }

        g.rules = (g.rules || []).filter(function(r) {
            return String((r && r.rule) || '').trim() !== '';
        });
        var existing = {};
        for (var i = 0; i < g.rules.length; i++) {
            existing[String(g.rules[i].rule || '').trim()] = 1;
        }

        var added = 0;
        for (var j = 0; j < parsed.length; j++) {
            if (existing[parsed[j]]) continue;
            var type = 'namespace';
            if (typeof window.mtDetectRuleType === 'function') {
                var detected = window.mtParseImportedRule(parsed[j]);
                parsed[j] = detected.rule; type = detected.type;
            }
            g.rules.push({
                id: _mtNewId(),
                name: '',
                type: type,
                rule: parsed[j],
                enable: true
            });
            existing[parsed[j]] = 1;
            added++;
        }
        if (!g.rules.length) {
            g.rules.push({ id: _mtNewId(), name: '', type: 'namespace', rule: '', enable: true });
        }
        if (added === 0) { _mtHint('Все правила уже есть в группе.'); return; }

        try { MT_STATE.dirty = true; } catch(e) {}
        /* Авторазворот свёрнутой группы, чтобы пользователь сразу увидел результат */
        _mtUncollapseGroup(ctx.gid);
        mtRenderGroups();
        window.closeMtBulkRulesModal();
        /* После rerender — ещё раз убедиться, что группа развёрнута (на случай гонок) */
        setTimeout(function() { _mtUncollapseGroup(ctx.gid); }, 0);
        if (typeof _mtSetStatus === 'function') {
            _mtSetStatus('Добавлено правил: ' + added, 'ok');
        }
        if (typeof window.__mtScanDups === 'function') setTimeout(window.__mtScanDups, 100);
        if (typeof window.__mtScanRuleTypes === 'function') setTimeout(window.__mtScanRuleTypes, 150);
    };

    window.mtApplyBulkRulesReplace = function() {
        var ctx = window.__mt_bulk_ctx;
        if (!ctx) { window.closeMtBulkRulesModal(); return; }
        var ta = document.getElementById('mt_bulk_rules_textarea');
        if (!ta) { window.closeMtBulkRulesModal(); return; }
        var parsed = _mtSplitRules(ta.value);

        if (!parsed.length) {
            if (!confirm('Список пуст. Очистить все правила в этой группе?')) return;
        }

        if (typeof _mtCollectFromUi === 'function') _mtCollectFromUi(true);
        var g = _mtFindGroupById(ctx.gid);
        if (!g) { window.closeMtBulkRulesModal(); return; }

        var newRules = [];
        for (var i = 0; i < parsed.length; i++) {
            var type = 'namespace';
            if (typeof window.mtDetectRuleType === 'function') {
                var detected = window.mtParseImportedRule(parsed[i]);
                parsed[i] = detected.rule; type = detected.type;
            }
            newRules.push({
                id: _mtNewId(),
                name: '',
                type: type,
                rule: parsed[i],
                enable: true
            });
        }
        if (!newRules.length) {
            newRules.push({ id: _mtNewId(), name: '', type: 'namespace', rule: '', enable: true });
        }
        g.rules = newRules;
        try { MT_STATE.dirty = true; } catch(e) {}
        /* Авторазворот свёрнутой группы */
        _mtUncollapseGroup(ctx.gid);
        mtRenderGroups();
        window.closeMtBulkRulesModal();
        setTimeout(function() { _mtUncollapseGroup(ctx.gid); }, 0);
        if (typeof _mtSetStatus === 'function') {
            _mtSetStatus('Заменено на ' + parsed.length + ' правил', 'ok');
        }
        if (typeof window.__mtScanDups === 'function') setTimeout(window.__mtScanDups, 100);
        if (typeof window.__mtScanRuleTypes === 'function') setTimeout(window.__mtScanRuleTypes, 150);
    };

    // Клик по кнопке в шапке группы
    document.addEventListener('click', function(e) {
        var t = e.target;
        if (!t || !t.closest) return;
        var btn = t.closest('.mt-group-bulk');
        if (!btn) return;
        var card = btn.closest('.mt-group-card');
        if (!card) return;
        var gid = card.getAttribute('data-gid');
        if (!gid) return;
        e.stopPropagation();
        window.mtOpenBulkRules(gid);
    }, true);

    // Горячие клавиши в модалке
    document.addEventListener('keydown', function(e) {
        var m = document.getElementById('mt_bulk_rules_modal');
        if (!m || m.style.display !== 'flex') return;
        if (e.key === 'Escape') {
            e.preventDefault();
            window.closeMtBulkRulesModal();
        } else if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'Enter' || e.keyCode === 13)) {
            e.preventDefault();
            window.mtApplyBulkRulesReplace();
        } else if ((e.ctrlKey || e.metaKey) && (e.key === 'Enter' || e.keyCode === 13)) {
            e.preventDefault();
            window.mtApplyBulkRules();
        }
    }, true);

    if (window.console && console.log) {
        console.log('optiray: MT bulk rules installed');
    }
})();


/* ==== Touch-based drag&drop для групп и правил ==== */
(function() {
    'use strict';
    if (window.__TOUCH_DND_INSTALLED) return;
    window.__TOUCH_DND_INSTALLED = true;

    var isTouch = ('ontouchstart' in window) || (navigator.maxTouchPoints > 0);
    if (!isTouch) return;

    var drag = null;

    /* --- Поиск drag-ручки под пальцем --- */
    function findHandle(x, y) {
        var el = document.elementFromPoint(x, y);
        if (!el || !el.closest) return null;

        // OptiRout: группа
        var h = el.closest('#mt_modal .mt-group-drag');
        if (h) {
            var c = h.closest('#mt_groups_container .mt-group-card');
            if (c) return { handle: h, item: c, kind: 'mt-group' };
        }
        // OptiRout: правило
        h = el.closest('#mt_modal .mt-rule-drag');
        if (h) {
            var r = h.closest('.mt-rule');
            if (r) return { handle: h, item: r, kind: 'mt-rule' };
        }
        // Routing: группа
        h = el.closest('#routing_modal .rt-group-drag');
        if (h) {
            var g = h.closest('.rt-group-card');
            if (g) return { handle: h, item: g, kind: 'rt-group' };
        }
        // Routing: правило
        h = el.closest('#routing_modal .rt-rule-drag');
        if (h) {
            var rr = h.closest('.rt-rule');
            if (rr) return { handle: h, item: rr, kind: 'rt-rule' };
        }
        return null;
    }

    /* --- Селектор "братьев" (куда можно бросить) --- */
    function sibSelector(kind) {
        if (kind === 'mt-group') return '#mt_groups_container > .mt-group-card';
        if (kind === 'mt-rule')  return '.mt-rules > .mt-rule';
        if (kind === 'rt-group') return '.rt-groups > .rt-group-card';
        if (kind === 'rt-rule')  return '.rt-rules > .rt-rule';
        return null;
    }

    /* --- Поиск цели под пальцем (исключая dragged элемент) --- */
    function findTarget(x, y, state) {
        var prevPE = state.item.style.pointerEvents;
        state.item.style.pointerEvents = 'none';
        var el = document.elementFromPoint(x, y);
        state.item.style.pointerEvents = prevPE || '';
        if (!el || !el.closest) return null;

        var sel = sibSelector(state.kind);
        if (!sel) return null;
        var t = el.closest(sel);
        if (!t || t === state.item) return null;
        if (t.parentNode !== state.item.parentNode) return null;
        return t;
    }

    function clearHints(kind) {
        var modal = (kind.indexOf('mt-') === 0) ? '#mt_modal' : '#routing_modal';
        var els = document.querySelectorAll(
            modal + ' .drag-over-top, ' + modal + ' .drag-over-bottom');
        for (var i = 0; i < els.length; i++) {
            els[i].classList.remove('drag-over-top');
            els[i].classList.remove('drag-over-bottom');
        }
    }

    function markDirty(kind) {
        if (kind.indexOf('mt-') === 0) {
            try { MT_STATE.dirty = true; } catch(e) {}
            try { _mtCollectFromUi(true); } catch(e) {}
            return;
        }
        var btn = document.getElementById('routing_save_btn');
        if (btn && !btn.classList.contains('routing-dirty')) {
            btn.classList.add('routing-dirty');
        }
    }

    /* --- touchstart --- */
    document.addEventListener('touchstart', function(e) {
        if (e.touches.length !== 1) return;
        var t = e.touches[0];
        var found = findHandle(t.clientX, t.clientY);
        if (!found) return;

        // Блокируем emulated mousedown/click по ручке —
        // иначе сработают HTML5-обработчики, которые только мешают
        if (e.cancelable) e.preventDefault();

        drag = {
            item: found.item,
            kind: found.kind,
            startX: t.clientX,
            startY: t.clientY,
            active: false,
            lastTarget: null
        };
    }, { passive: false, capture: true });

    /* --- touchmove --- */
    document.addEventListener('touchmove', function(e) {
        if (!drag) return;
        if (e.touches.length !== 1) return;
        var t = e.touches[0];

        // Если DOM перерисовался (например, periodic scan) — прекращаем drag
        if (!drag.item || !drag.item.isConnected) {
            drag = null;
            return;
        }

        // Активация после сдвига >6px
        if (!drag.active) {
            var dx = t.clientX - drag.startX;
            var dy = t.clientY - drag.startY;
            if (Math.abs(dx) < 6 && Math.abs(dy) < 6) return;
            drag.active = true;
            drag.item.classList.add('dragging');
            drag.item.style.pointerEvents = 'none';
            if (navigator.vibrate) {
                try { navigator.vibrate(12); } catch(_) {}
            }
        }

        if (e.cancelable) e.preventDefault();

        clearHints(drag.kind);
        var target = findTarget(t.clientX, t.clientY, drag);
        if (target) {
            var rect = target.getBoundingClientRect();
            var after = t.clientY > rect.top + rect.height / 2;
            target.classList.add(after ? 'drag-over-bottom' : 'drag-over-top');
            var parent = target.parentNode;
            parent.insertBefore(drag.item,
                after ? target.nextSibling : target);
            drag.lastTarget = target;
        }
    }, { passive: false, capture: true });

    /* --- touchend / touchcancel --- */
    function endTouch() {
        if (!drag) return;
        var d = drag;
        drag = null;
        if (d.active) {
            clearHints(d.kind);
            if (d.item) {
                d.item.classList.remove('dragging');
                d.item.style.pointerEvents = '';
            }
            markDirty(d.kind);
            if (navigator.vibrate) {
                try { navigator.vibrate(8); } catch(_) {}
            }
        }
    }
    document.addEventListener('touchend', endTouch, { passive: true, capture: true });
    document.addEventListener('touchcancel', endTouch, { passive: true, capture: true });

    if (window.console && console.log) {
        console.log('optiray: touch DnD installed (groups + rules)');
    }
})();

function selectSettingsTab(key) {
    if (key === "routing") key = "connection";
    var modal = document.getElementById('settings_modal');
    if (!modal) return;
    window.optiraySettingsTab = key;
    modal.querySelectorAll('.settings-panel').forEach(function(panel) {
        panel.hidden = panel.id !== 'settings_panel_' + key;
    });
    modal.querySelectorAll('[data-settings-tab]').forEach(function(button) {
        if (button.dataset.settingsTab === key) button.setAttribute('aria-current', 'page');
        else button.removeAttribute('aria-current');
    });
    modal.querySelector('.settings-scroll').scrollTop = 0;
    if (key === 'cores') { loadXrayVersions(); checkHysteria(); loadGeoData(); }
}
document.addEventListener('keydown', function(event) {
    var modal = document.getElementById('settings_modal');
    if (!modal || modal.style.display !== 'flex') return;
    if (event.key === 'Escape') { closeSettingsModal(); return; }
    if (event.key !== 'Tab') return;
    var items = Array.from(modal.querySelectorAll('button, input, select, textarea, a[href]')).filter(function(el) {
        return !el.disabled && el.getClientRects().length > 0;
    });
    if (!items.length) return;
    var first = items[0], last = items[items.length - 1];
    if (event.shiftKey && (document.activeElement === first || !modal.contains(document.activeElement))) {
        event.preventDefault(); last.focus();
    } else if (!event.shiftKey && (document.activeElement === last || !modal.contains(document.activeElement))) {
        event.preventDefault(); first.focus();
    }
});

function mtFilterGroups() {
 if(document.getElementById('mt_modal')?.dataset.ruleEngine==='xray'){syncSharedXraySearch();return;}
 var cont = document.getElementById('mt_groups_container');
 if (!cont) return;
 var search = document.getElementById('mt_group_search');
 var filter = document.getElementById('mt_group_filter');
 var query = (search ? search.value : '').trim().toLowerCase();
 var mode = filter ? filter.value : 'all';
 var cards = cont.querySelectorAll('.mt-group-card');
 var visible = 0;
 cards.forEach(function(card) {
  var name = card.querySelector('.mt-group-name');
  var check = card.querySelector('.mt-group-enable');
  var enabled = !check || check.checked;
  var ruleInputs = Array.from(card.querySelectorAll('.mt-rule-input'));
  var values = ruleInputs.map(function(input) { return input.value; });
  var ruleMatch = ruleInputs.some(function(input) { return query && input.value.toLowerCase().includes(query); });
  ruleInputs.forEach(function(input) { input.classList.toggle('routing-search-match', !!query && input.value.toLowerCase().includes(query)); });
  if(name)name.classList.toggle('routing-search-match', !!query && name.value.toLowerCase().includes(query));
  if(query && !card.dataset.searchCollapseSaved){
   card.dataset.searchCollapseSaved='1';card.dataset.searchWasCollapsed=String(card.classList.contains('collapsed'));
  }
  if(query && ruleMatch){card.classList.remove('collapsed');}
  if(!query && card.dataset.searchCollapseSaved){
   card.classList.toggle('collapsed',card.dataset.searchWasCollapsed==='true');
   delete card.dataset.searchCollapseSaved;delete card.dataset.searchWasCollapsed;
  }
  var toggle=card.querySelector('.mt-group-toggle');
  if(toggle){toggle.title=card.classList.contains('collapsed')?'Развернуть':'Свернуть';toggle.setAttribute('aria-expanded',String(!card.classList.contains('collapsed')));}
  var text = ((name ? name.value : '') + ' ' + values.join(' ')).toLowerCase();
  card.hidden = !!((query && text.indexOf(query) < 0) || (mode === 'enabled' && !enabled) || (mode === 'disabled' && enabled) || (mode === 'blocked' && card.querySelector('.mt-group-iface')?.value !== 'blackhole'));
  if (!card.hidden) visible++;
  var head = card.querySelector('.mt-group-head');
  if (head && name) {
   var badge = head.querySelector('.mt-group-rule-count');
   if (!badge) { badge = document.createElement('span'); badge.className = 'mt-group-rule-count'; name.after(badge); }
   var count = values.filter(function(value) { return value.trim(); }).length;
   var n = count % 100, last = count % 10;
   badge.textContent = count + ' ' + (n >= 11 && n <= 14 ? 'правил' : last === 1 ? 'правило' : last >= 2 && last <= 4 ? 'правила' : 'правил');
  }
 });
 var summary = document.getElementById('mt_filter_count');
 if (summary) summary.textContent = visible + '/' + cards.length;
 var empty = document.getElementById('mt_filter_empty');
 if (empty) empty.hidden = cards.length === 0 || visible > 0;
}
document.addEventListener('DOMContentLoaded', function() {
 var cont = document.getElementById('mt_groups_container');
 if (!cont) return;
 new MutationObserver(mtFilterGroups).observe(cont, {childList:true});
});
document.addEventListener('change', function(event) {
 if (event.target.matches('.mt-group-enable')) mtFilterGroups();
});

(function() {
 var trigger = null, menu = null;
 window._mtCloseGroupIface = close;
 function close() {
  if (menu) menu.style.display = 'none';
  if (trigger) {
   trigger.setAttribute('aria-expanded', 'false');
   trigger.classList.remove('is-menu-attached');
   delete trigger.dataset.menuSide;
  }
  trigger = null;
 }
 function open(button) {
  if (trigger === button) { close(); return; }
  close();
  if (typeof _mtCloseGlobalIface === 'function') _mtCloseGlobalIface();
  trigger = button;
  var select = button.closest('.mt-iface-picker').querySelector('.mt-group-iface');
  if (!menu) {
   menu = document.createElement('div'); menu.id = 'mt_group_iface_dd';
   menu.className = 'mt-global-iface-dd is-attached'; menu.setAttribute('role','menu');
   menu.setAttribute('aria-label','Интерфейс группы'); document.body.appendChild(menu);
  }
  menu.replaceChildren();
  Array.from(select.options).forEach(function(option) {
   var item = document.createElement('button'); item.type = 'button';
   item.className = 'mt-group-iface-dd-item' + (option.selected ? ' is-current' : '');
   item.setAttribute('role','menuitemradio'); item.setAttribute('aria-checked', String(option.selected));
   var name = document.createElement('div'); name.className = 'mt-global-iface-dd-name';
   name.textContent = option.dataset.name || option.value || 'Не выбрано'; item.appendChild(name);
   if (option.dataset.desc) {
    var desc = document.createElement('div'); desc.className = 'mt-global-iface-dd-desc';
    desc.textContent = option.dataset.desc; item.appendChild(desc);
   }
   item.addEventListener('click', function() {
    select.value = option.value; select.dispatchEvent(new Event('change', {bubbles:true}));
    var restore = trigger; close(); if (restore) restore.focus();
   }); menu.appendChild(item);
  });
  button.classList.add('is-menu-attached');
  var group = button.closest('.mt-group-controls');
  var rect = (group || button).getBoundingClientRect();
  menu.style.display = 'block';
  menu.style.width = 'max-content';
  menu.style.maxHeight = Math.min(300,innerHeight - 24) + 'px';
  var optionWidth = Array.from(menu.querySelectorAll('.mt-global-iface-dd-name,.mt-global-iface-dd-desc')).reduce(function(width,label) {
   return Math.max(width,label.scrollWidth + 48);
  },0);
  var width = Math.min(Math.max(button.offsetWidth + 2,220,optionWidth),innerWidth - 24);
  menu.style.width = width + 'px';
  var below = Math.max(0,innerHeight - rect.bottom - 12);
  var above = Math.max(0,rect.top - 12);
  var side = menu.offsetHeight <= below || below >= above ? 'below' : 'above';
  menu.dataset.side = side;
  button.dataset.menuSide = side;
  menu.style.maxHeight = Math.min(300,side === 'below' ? below : above) + 'px';
  menu.style.left = Math.max(12,Math.min(rect.left,innerWidth - width - 12)) + 'px';
  menu.style.top = (side === 'below' ? rect.bottom - 1 : rect.top - menu.offsetHeight + 1) + 'px';
  button.setAttribute('aria-expanded','true');
  (menu.querySelector('[aria-checked="true"]') || menu.firstElementChild).focus();
 }
 document.addEventListener('click', function(event) {
  var button = event.target.closest('#mt_modal button.mt-iface-display');
  if (button) { open(button); return; }
  if (!menu || !menu.contains(event.target)) close();
 });
 document.addEventListener('keydown', function(event) {
  if (!trigger || !menu || menu.style.display === 'none') return;
  if (event.key === 'Escape') { var restore = trigger; close(); restore.focus(); event.preventDefault(); }
  if (event.key === 'Tab') close();
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
   var items = Array.from(menu.querySelectorAll('button'));
   var i = items.indexOf(document.activeElement);
   items[(i + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length].focus(); event.preventDefault();
  }
 });
 window.addEventListener('resize',close);
 window.addEventListener('scroll',function(event) { if (!menu || !menu.contains(event.target)) close(); },true);
})();

function setServerView(view) {
 var root=document.getElementById('servers_container'); if(!root)return;
 root.classList.toggle('server-view-list',view!=='cards'); root.classList.toggle('server-view-cards',view==='cards');
 document.querySelectorAll('[data-server-view]').forEach(function(b){b.setAttribute('aria-pressed',String(b.dataset.serverView===view));});
 try {localStorage.setItem('optiray_server_view',view);}catch(e){}
}
function browseServers() {
 var root=document.getElementById('servers_container');if(!root)return;
 var query=document.getElementById('server_search').value.trim().toLowerCase();
 var filter=document.getElementById('server_filter').value,sort=document.getElementById('server_sort').value;
 root.classList.toggle('server-browse-active',!!query||filter!=='all');
 var visible=0,total=0;
 root.querySelectorAll('.sub-section').forEach(function(section){
  var grid=section.querySelector('.server-grid'); if(!grid)return;
  var cards=Array.from(grid.querySelectorAll('.server-card')),matched=0;
  function latency(card){var badge=card.querySelector('.ping-badge');var value=badge?parseFloat(badge.textContent):NaN;return Number.isFinite(value)&&value>0?value:Infinity;}
  cards.forEach(function(card){
   var match=(!query||(card.dataset.name+' '+section.dataset.subname).toLowerCase().includes(query))&&(filter!=='favorites'||card.dataset.fav==='1')&&(filter!=='available'||latency(card)!==Infinity);
   card.classList.toggle('browse-hidden',!match);total++;if(match){matched++;visible++;}
  });
  section.classList.toggle('browse-hidden',matched===0);
  cards.sort(function(a,b){var active=Number(b.dataset.active==='1')-Number(a.dataset.active==='1');if(active)return active;if(sort==='name')return a.dataset.name.localeCompare(b.dataset.name,'ru');if(sort==='latency')return latency(a)-latency(b)||Number(a.dataset.browseOrder)-Number(b.dataset.browseOrder);return Number(b.dataset.fav==='1')-Number(a.dataset.fav==='1')||Number(a.dataset.browseOrder)-Number(b.dataset.browseOrder);});
  cards.forEach(function(card){grid.appendChild(card);});
 });
 document.getElementById('server_results_count').textContent=visible+'/'+total;
 document.getElementById('server_results_empty').hidden=visible>0||total===0;
}
document.addEventListener('DOMContentLoaded',function(){
 var root=document.getElementById('servers_container');if(!root)return;
 var toolbar=document.querySelector('.server-browser-toolbar');if(toolbar)toolbar.id='server_browser_toolbar';
 root.querySelectorAll('.sub-section').forEach(function(section){
  var color=section.dataset.subcolor;if(/^#[0-9a-f]{6}$/i.test(color||''))section.style.setProperty('--subscription-color',color);
  var fav=section.querySelector('.btn-sub-fav-ping');if(fav){fav.title='HTTP-проверка избранных серверов';fav.setAttribute('aria-label',fav.title);fav.lastChild.textContent='Избранные';}
 });
 root.querySelectorAll('.server-card').forEach(function(card,index){
  card.dataset.browseOrder=card.dataset.sourceOrder!==undefined?card.dataset.sourceOrder:index;
  var name=card.querySelector('.server-name'),ping=card.querySelector('.ping-badge'),subtitle=card.querySelector('.server-subtitle'),action=card.querySelector('.action-slot');
  if(!name||!ping||!subtitle||!action)return;
  var identity=name.parentElement;identity.className='server-identity';identity.removeAttribute('style');
  var main=document.createElement('div');main.className='server-row-main';main.append(identity,subtitle);
  card.replaceChildren(main,ping,action);ping.title='Нажмите для HTTP-проверки задержки';
 });
 var view='list';try{view=localStorage.getItem('optiray_server_view')||'list';}catch(e){}setServerView(view==='cards'?'cards':'list');browseServers();
 var scheduled=false;
 new MutationObserver(function(records){
  if(!records.some(function(r){var el=r.target.nodeType===1?r.target:r.target.parentElement;return el&&(el.closest('.ping-badge')||(r.type==='attributes'&&(r.attributeName==='data-fav'||r.attributeName==='data-active')));}))return;
  if(scheduled)return;scheduled=true;setTimeout(function(){scheduled=false;browseServers();},250);
 }).observe(root,{subtree:true,childList:true,characterData:true,attributes:true,attributeFilter:['data-fav','data-active']});
});

function serverChevron(collapsed) {
 return '<svg class="server-chevron'+(collapsed?'':' is-open')+'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m9 5 7 7-7 7"/></svg>';
}
function renderSubscriptionToggle(section, button, closed) {
 var sibling=button.nextElementSibling;
 var oldName=button.querySelector('.subscription-toggle-name');
 var label=section.dataset.subname || (oldName && oldName.textContent) ||
  (sibling && sibling.tagName==='SPAN' && !sibling.className ? sibling.textContent : '');
 var name=document.createElement('span');
 name.className='subscription-toggle-name';
 name.textContent=label;
 button.innerHTML=serverChevron(closed);
 button.appendChild(name);
 if(sibling && sibling.tagName==='SPAN' && !sibling.className) sibling.remove();
 button.title=closed?'Развернуть подписку':'Свернуть подписку';
 button.setAttribute('aria-label',button.title+': '+label);
 button.setAttribute('aria-expanded',String(!closed));
}
document.addEventListener('DOMContentLoaded',function(){
 var root=document.getElementById('servers_container');if(!root)return;
 root.querySelectorAll('.sub-section').forEach(function(section){
  var button=section.querySelector('.btn-toggle-collapse');
  var test=section.querySelector('.btn-test-site'),actions=section.querySelector('.sub-actions');
  if(test&&actions)actions.prepend(test);
  if(button){
   var closed=section.classList.contains('collapsed');
   renderSubscriptionToggle(section, button, closed);
  }
 });
 var icons=[
 ['.btn-sub-fav-ping','HTTP-проверка избранных','<path transform="translate(1.8 1.2) scale(.85)" d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/>'],
 ['.btn-sub-tcppool','TCP-проверка соединения','<path d="M7 8h10M7 16h10M3 3v18M21 3v18"/><path d="m14 5 3 3-3 3m-4 2-3 3 3 3"/>'],
 ['.btn-sub-ping','HTTP-проверка задержки','<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c5 5 5 13 0 18-5-5-5-13 0-18Z"/>'],
 ['.btn-sub-stop','Остановить проверки','<rect x="3" y="3" width="18" height="18" rx="3"/>']
 ];
 icons.forEach(function(item){root.querySelectorAll(item[0]).forEach(function(button){button.innerHTML='<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">'+item[2]+'</svg>';button.title=item[1];button.setAttribute('aria-label',item[1]);});});
});

document.addEventListener('DOMContentLoaded',function(){
 document.querySelectorAll('.sub-row').forEach(function(row,index){
  var input=row.querySelector('input[type="color"]');if(!input)return;
  row.style.setProperty('--subscription-color',input.value);
  var chip=document.querySelectorAll('.subs-name-chip')[index];if(chip)chip.style.setProperty('--subscription-color',input.value);
 });
});

document.addEventListener('toggle',function(event){
 var details=event.target;
 if(!details.matches || !details.matches('#mt_modal .mt-maintenance') || !details.open)return;
 requestAnimationFrame(function(){
  var scroller=details.closest('.rt-routing-root')||details.closest('.unified-xray-editor')||details.closest('.mt-root');if(!scroller)return;
  var box=scroller.getBoundingClientRect(),panel=details.getBoundingClientRect();
  var delta=panel.height>scroller.clientHeight-24 ? panel.top-box.top-12 : panel.bottom-box.bottom+12;
  if(delta>0)scroller.scrollTo({top:scroller.scrollTop+delta,behavior:matchMedia('(prefers-reduced-motion:reduce)').matches?'auto':'smooth'});
 });
},true);

document.addEventListener('input',function(event){
 var input=event.target;
 if(!input.matches('.mt-group-color,.xr-group-color'))return;
 var card=input.closest('.mt-group-card,.rt-group-card');
 if(card)card.style.setProperty('--rule-group-color',input.value);
});

document.addEventListener('DOMContentLoaded',function(){
 document.querySelectorAll('#servers_container .fav-btn').forEach(function(button){
  var path=button.querySelector('path');if(path)path.setAttribute('d','M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z');
  button.title='Добавить или убрать из избранного';button.setAttribute('aria-label',button.title);
 });
 document.querySelectorAll('.sub-row .dice-btn').forEach(function(button){
  button.innerHTML=OPTIRAY_AUTO_COLOR_ICON;
  button.title='Выбрать другой случайный цвет';button.setAttribute('aria-label',button.title);
 });
});

/* Сохраняем native select как источник значения и существующих change-обработчиков. */
(function(){
 var menu,trigger;
 function close(restore){
  if(menu){menu.hidden=true;menu.style.display='none';}
  if(trigger){var old=trigger;old.setAttribute('aria-expanded','false');old.classList.remove('is-menu-attached');delete old.dataset.menuSide;trigger=null;if(restore)old.focus();}
 }
 function open(button,select){
  if(trigger===button){close(true);return;}
  close(false);
  if(typeof _mtCloseGlobalIface==='function')_mtCloseGlobalIface();
  if(typeof _mtCloseGroupIface==='function')_mtCloseGroupIface();
  trigger=button;
  if(!menu){menu=document.createElement('div');menu.id='optiray_select_dd';menu.setAttribute('role','menu');document.body.appendChild(menu);}
  var ruleType = select.matches('select.mt-rule-type');
  var attached = ruleType || select.id === 'mt_group_filter' || select.id === 'xr_group_filter' || select.id === 'server_filter' || select.id === 'server_sort' || !!select.closest('#settings_modal');
  menu.classList.toggle('is-attached',attached);
  menu.classList.toggle('is-rule-type',ruleType);
  menu.style.removeProperty('width');
  delete menu.dataset.side;
  menu.replaceChildren();menu.setAttribute('aria-label',select.getAttribute('aria-label')||'Выбор');
  Array.from(select.options).forEach(function(option){
   if(option.dataset.separator === 'true'){var separator=document.createElement('div');separator.className='optiray-select-separator';separator.setAttribute('role','separator');menu.appendChild(separator);}
   var item=document.createElement('button');item.type='button';
   item.className='mt-group-iface-dd-item'+(option.selected?' is-current':'');
   item.disabled=option.disabled;item.setAttribute('role','menuitemradio');item.setAttribute('aria-checked',String(option.selected));
   var name=document.createElement('div');name.className='mt-global-iface-dd-name';name.textContent=option.textContent;item.appendChild(name);
   item.addEventListener('click',function(){select.value=option.value;select.dispatchEvent(new Event('change',{bubbles:true}));close(true);});
   menu.appendChild(item);
  });
  menu.hidden=false;menu.style.display='block';
  var fullList = select.id === 'xray_version_select';
  menu.style.maxHeight = fullList ? 'none' : Math.min(300,innerHeight-24)+'px';
  menu.style.overflowY = fullList ? 'visible' : 'auto';
  var anchor=ruleType?(button.closest('.mt-rule-controls')||button):button;
  var rect=anchor.getBoundingClientRect(),height=menu.offsetHeight;
  if(attached){
   // Press feedback scales the button; anchor to its unscaled layout box.
   var width=anchor.offsetWidth,buttonHeight=anchor.offsetHeight;
   var left=rect.left-(width-rect.width)/2,top=rect.top-(buttonHeight-rect.height)/2,bottom=top+buttonHeight;
   // Keep the trigger sized to its label while allowing all options to fit.
   menu.style.width='max-content';
   var optionWidth=Array.from(menu.querySelectorAll('.mt-global-iface-dd-name')).reduce(function(w,name){return Math.max(w,name.scrollWidth+48);},0);
   width=Math.min(Math.max(width,optionWidth),innerWidth-24);
   menu.style.width=width+'px';
   left=Math.max(12,Math.min(left,innerWidth-width-12));
   var below=Math.max(0,innerHeight-bottom-12),above=Math.max(0,top-12);
   var side=below>=menu.scrollHeight||below>=above?'below':'above';
   menu.dataset.side=side;button.dataset.menuSide=side;button.classList.add('is-menu-attached');
   menu.style.maxHeight=(side==='below'?below:above)+'px';
   height=menu.offsetHeight;
   menu.style.left=left+'px';
   menu.style.top=(side==='below'?bottom-(ruleType?1:0):top-height+(ruleType?1:0))+'px';
  }else{
   menu.style.left=Math.max(12,Math.min(rect.left,innerWidth-menu.offsetWidth-12))+'px';
   menu.style.top=Math.max(12,rect.bottom+height+8<innerHeight?rect.bottom+6:rect.top-height-6)+'px';
  }
  button.setAttribute('aria-expanded','true');
  (menu.querySelector('[aria-checked="true"]')||menu.firstElementChild).focus();
 }
 window.optirayEnhanceSelect=function(select){
   if(!select||select.dataset.optirayEnhanced)return;
   select.dataset.optirayEnhanced='1';
   var button=document.createElement('button');button.type='button';button.className='optiray-select-trigger';button.dataset.selectId=select.id;
   var filterCount=select.id==='mt_group_filter'?document.getElementById('mt_filter_count'):select.id==='xr_group_filter'?document.getElementById('xr_filter_count'):select.id==='server_filter'?document.getElementById('server_results_count'):null;
   button.setAttribute('aria-label',select.getAttribute('aria-label'));button.setAttribute('aria-haspopup','menu');button.setAttribute('aria-expanded','false');button.setAttribute('aria-controls','optiray_select_dd');
   function sync(){button.replaceChildren();var text=document.createElement('span');text.textContent=(select.selectedOptions[0]||select.options[0]||{textContent:''}).textContent;button.appendChild(text);if(filterCount)button.appendChild(filterCount);button.insertAdjacentHTML('beforeend','<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m6 9 6 6 6-6"/></svg>');}
   select.hidden=true;select.style.display='none';select.after(button);sync();if(select.matches('.mt-rule-type')){var ctx=document.createElement('canvas').getContext('2d');ctx.font=getComputedStyle(button).font;var widest=Math.max.apply(null,Array.from(select.options).map(function(option){return ctx.measureText(option.textContent).width;}));button.style.setProperty('--rule-type-width',Math.ceil(widest+34)+'px');}select.addEventListener('change',sync);new MutationObserver(sync).observe(select,{childList:true,subtree:true,characterData:true});
   button.addEventListener('click',function(e){e.stopPropagation();open(button,select);});
   button.addEventListener('keydown',function(e){if(e.key==='ArrowDown'||e.key==='ArrowUp'){e.preventDefault();open(button,select);}});
 };
 document.addEventListener('DOMContentLoaded',function(){
  ['server_filter','server_sort','mt_group_filter','xray_version_select','hysteria_version_select','subscription_client_profile'].forEach(function(id){window.optirayEnhanceSelect(document.getElementById(id));});
  function enhanceRuleTypes(root){
   if(root.nodeType!==1 && root.nodeType!==9)return;
   if(root.matches && root.matches('select.mt-rule-type'))window.optirayEnhanceSelect(root);
   root.querySelectorAll('select.mt-rule-type').forEach(window.optirayEnhanceSelect);
  }
  enhanceRuleTypes(document);
  new MutationObserver(function(records){
   records.forEach(function(record){record.addedNodes.forEach(enhanceRuleTypes);});
  }).observe(document.getElementById('mt_modal'),{childList:true,subtree:true});
 });
 document.addEventListener('click',function(e){if(trigger&&!menu.contains(e.target)&&!trigger.contains(e.target))close(false);});
 document.addEventListener('keydown',function(e){
  if(!trigger||!menu||menu.hidden)return;
  if(e.key==='Escape'){e.preventDefault();close(true);}
  else if(e.key==='Tab')close(false);
  else if(e.key==='ArrowDown'||e.key==='ArrowUp'||e.key==='Home'||e.key==='End'){
   var items=Array.from(menu.querySelectorAll('button:not(:disabled)')),i=items.indexOf(document.activeElement);
   i=e.key==='Home'?0:e.key==='End'?items.length-1:(i+(e.key==='ArrowDown'?1:-1)+items.length)%items.length;
   items[i].focus();e.preventDefault();
  }
 });
 window.addEventListener('resize',function(){close(false);});
 window.addEventListener('scroll',function(e){if(!menu||!menu.contains(e.target))close(false);},true);
})();

document.addEventListener('DOMContentLoaded',function(){
 var card=document.querySelector('.header-right > .active-status-card');
 if(!card)return;
 var anchor=card.parentElement,pinned=false,scheduled=false;
 function release(){
  card.classList.remove('is-pinned');card.style.removeProperty('width');card.style.removeProperty('right');
  anchor.style.removeProperty('width');anchor.style.removeProperty('height');pinned=false;
 }
 function update(){
  scheduled=false;
  var rect=anchor.getBoundingClientRect();
  if(rect.top<12){
   if(!pinned){
    var size=card.getBoundingClientRect();
    anchor.style.width=rect.width+'px';anchor.style.height=rect.height+'px';
    card.style.width=size.width+'px';card.classList.add('is-pinned');pinned=true;
   }
   card.style.right=Math.max(12,innerWidth-rect.right)+'px';
  }else if(pinned)release();
 }
 window.addEventListener('scroll',function(){if(!scheduled){scheduled=true;requestAnimationFrame(update);}},{passive:true});
 window.addEventListener('resize',function(){release();update();});
 var contentResizeQueued=false;
 new MutationObserver(function(){
  if(contentResizeQueued)return;
  contentResizeQueued=true;
  requestAnimationFrame(function(){
   contentResizeQueued=false;
   release();
   update();
  });
 }).observe(card,{childList:true,characterData:true,subtree:true});
 update();
});

function syncXrayOutboundOrder() {
 var editor=document.getElementById('routing_modal');if(!editor)return;
 var checked=editor.querySelector('input[name="routing_mode"]:checked');
 var sections={};
 ['block','direct','proxy'].forEach(function(cat){
  var groups=editor.querySelector('#rt_groups_'+cat);
  if(groups)sections[cat]=groups.closest('.xr-outbound')||groups.closest('.setting-field');
 });
 if(!sections.block||!sections.direct||!sections.proxy)return;
 var parent=sections.block.parentNode;
 if(sections.direct.parentNode!==parent||sections.proxy.parentNode!==parent)return;
 var order=checked&&checked.value==='direct_default'?['proxy','direct']:['direct','proxy'];
 parent.insertBefore(sections[order[0]],sections.block.nextSibling);
 parent.insertBefore(sections[order[1]],sections[order[0]].nextSibling);
}
document.addEventListener('change',function(event){
 if(event.target.matches('#routing_modal input[name="routing_mode"]'))syncXrayOutboundOrder();
});

(function(){
 var mode='optirout',xrayDirty=false,busy=false,xrayBaseline=null;
 function xraySignature(){
  var editor=document.getElementById('routing_modal'),routing={groups_ui:window.__rt_get_data?window.__rt_get_data():{}};
  var order=editor.querySelector('[name="routing_mode"]:checked');routing.direct_default=!!(order&&order.value==='direct_default');
  ['block','direct','proxy'].forEach(function(cat){routing[cat+'_rules']=editor.querySelector('[name="routing_'+cat+'_rules"]').value;});
  return routingConfigSignature('xray',routing);
 }
 var originalOpenMt=openMtModal,originalCloseMt=closeMtModal;
 function refresh(){
  var modal=document.getElementById('mt_modal');if(!modal)return;
  if(mode==='xray'&&xrayBaseline!==null)xrayDirty=xraySignature()!==xrayBaseline;
  modal.dataset.ruleEngine=mode; syncRoutingHeaderActions(mode);
  var heading=document.getElementById('mt_dialog_title');
  if(heading && heading.tagName!=='BUTTON') {
   var titleButton=document.createElement('button');
   titleButton.type='button';titleButton.id='mt_dialog_title';
   titleButton.className='power-status-button routing-title-power';
   titleButton.innerHTML='<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="M12 3v9M6.4 5.6a8 8 0 1 0 11.2 0"/></svg><span></span><span class="power-status-dot" aria-hidden="true"></span>';
   titleButton.onclick=function(){powerButtonClick(this);};
   heading.replaceWith(titleButton);heading=titleButton;
  }
  if(heading){heading.dataset.powerTarget=mode==='xray'?'xray_routing_enabled':'mt_enabled';syncPowerButtons();}
  var editor=document.getElementById('routing_modal');if(editor)editor.hidden=mode!=='xray';
  if(editor)editor.inert=busy;
  var reload=document.getElementById('xr_reload_btn');if(reload)reload.disabled=busy;
  modal.querySelectorAll('[data-rule-mode]').forEach(function(b){b.setAttribute('aria-pressed',String(b.dataset.ruleMode===mode));b.disabled=busy;});
  var save=document.getElementById('mt_save_btn');
  if(save&&mode==='xray'){save.disabled=busy||!xrayDirty;save.setAttribute('aria-disabled',String(save.disabled));save.classList.toggle('mt-save-idle',!xrayDirty);save.classList.remove('mt-blocked','mt-has-dups');save.style.opacity='';}
  if(save&&mode==='optirout'){_mtSetSaveDirty(MT_STATE.dirty);if(window.__mtRefreshSaveBtn)window.__mtRefreshSaveBtn();}
  if(save&&busy){save.disabled=true;save.setAttribute('aria-disabled','true');}
  if(mode==='xray'){var notice=document.getElementById('mt_dirty_notice');if(notice)notice.textContent=xrayDirty?'Есть несохранённые изменения':'';_mtSetStatus(xrayDirty?'Есть несохранённые изменения':'Правила Xray','ok');}
  else if(MT_STATE.config)_mtSetStatus('Конфиг загружен. '+ruCount((MT_STATE.config.groups||[]).length,'группа','группы','групп'),'ok');
 }
 async function loadXrayConfig(){
  var response=await fetch('/api/xray_routing_config',{cache:'no-store'});
  var payload=await response.json();
  if(!response.ok||!payload.ok)throw new Error(payload.error||'Не удалось загрузить правила Xray');
  var backupToggle=document.getElementById('xr_backup_enabled');if(backupToggle)backupToggle.checked=payload.save_backups===true;
  var routing=payload.routing;
  if(!routing||typeof routing!=='object'||Array.isArray(routing))throw new Error('Некорректная конфигурация Xray');
  var groups=JSON.parse(JSON.stringify(routing.groups_ui||{}));
  ['block','direct','proxy'].forEach(function(cat){
   if(groups[cat]!==undefined&&!Array.isArray(groups[cat]))throw new Error('Некорректный список групп Xray');
   if(routing[cat+'_rules']!==undefined&&typeof routing[cat+'_rules']!=='string')throw new Error('Некорректный список правил Xray');
   (groups[cat]||[]).forEach(function(group){if(group&&typeof group==='object')group.collapsed=true;});
  });
  if(!window.__rt_set_data)throw new Error('Редактор правил ещё не готов');
  var editor=document.getElementById('routing_modal');
  editor.querySelector('[name="routing_enabled"]').checked=!!routing.enabled;syncPowerButtons();
  editor.querySelectorAll('[name="routing_mode"]').forEach(function(input){input.checked=input.value===(routing.direct_default?'direct_default':'proxy_default');});
  ['block','direct','proxy'].forEach(function(cat){editor.querySelector('[name="routing_'+cat+'_rules"]').value=routing[cat+'_rules']||'';});
  window.__rt_set_data(groups,{clean:true});syncXrayOutboundOrder();xrayBaseline=xraySignature();xrayDirty=false;
  if(window.refreshXrayGeoCategories)window.refreshXrayGeoCategories();
  if(window.filterXrayGroups)window.filterXrayGroups();
  if(window._optirayRoutingKick)window._optirayRoutingKick();
 }
 window.setRoutingPower=async function(toggle,engine){
  var wanted=toggle.checked,previous=!wanted;
  if(busy){toggle.checked=previous;syncPowerButtons();return;}
  busy=true;toggle.disabled=true;refresh();syncPowerButtons();
  try{
   var response=await fetch('/api/routing_power',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({engine:engine,enabled:wanted})});
   var result=await response.json();if(!response.ok||!result.ok)throw new Error(result.error||'Не удалось переключить маршрутизацию');
   toggle.checked=result.enabled;
   if(engine==='optirout'&&MT_STATE.config)MT_STATE.config.enabled=result.enabled;
  }catch(error){toggle.checked=previous;alert(error.message);}
  finally{busy=false;toggle.disabled=false;if(engine==='optirout')mtUpdatePowerUi();refresh();syncPowerButtons();}
 };
 window.setXrayBackupEnabled=async function(toggle){
  if(busy)return;
  var previous=!toggle.checked,error;busy=true;refresh();
  try{
   var response=await fetch('/api/xray_backup_settings',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({enabled:toggle.checked})});
   var result=await response.json();if(!response.ok||!result.ok)throw new Error(result.error||'Не удалось сохранить настройку');
   toggle.checked=result.enabled;
  }catch(e){toggle.checked=previous;error=e.message;}
  finally{busy=false;refresh();if(error)_mtSetStatus(error,'err');}
 };
 window.resetXrayRules=async function(){
  if(busy||!confirm('Сбросить все группы и правила Xray? Конфигурация маршрутизации будет восстановлена по умолчанию.'))return;
  var error;busy=true;refresh();_mtSetStatus('Сброс правил Xray...','');
  try{
   var response=await fetch('/api/xray_reset',{method:'POST'});var result=await response.json();
   if(!response.ok||!result.ok)throw new Error(result.error||'Не удалось сбросить правила');
   await loadXrayConfig();
  }catch(e){error=e.message;}
  finally{busy=false;refresh();if(error)_mtSetStatus(error,'err');else _mtSetStatus('Правила Xray сброшены','ok');}
 };
 window.reloadXrayConfig=async function(){
  if(busy)return;
  if(xrayDirty&&!confirm('Обновить правила из файла? Несохранённые изменения будут потеряны.'))return;
  var error;
  busy=true;refresh();_mtSetStatus('Загрузка правил Xray...','');
  try{await loadXrayConfig();}catch(e){error=e.message;}
  finally{busy=false;refresh();if(error)_mtSetStatus(error,'err');}
 };
 async function openUnified(){
  var searchModal=document.getElementById('mt_modal');
  if(searchModal && searchModal.style.display!=='flex'){
   ['mt_group_search','xr_group_search'].forEach(function(id){var input=document.getElementById(id);if(input)input.value='';});
   mtFilterGroups();if(window.filterXrayGroups)window.filterXrayGroups();
  }
  if(mode==='optirout')await originalOpenMt();
  else{
   var modal=document.getElementById('mt_modal');
   if(modal.style.display==='flex')return;
   modal.style.display='flex';await window.reloadXrayConfig();return;
  }
  refresh();
  if(mode==='xray'&&window._optirayRoutingKick)window._optirayRoutingKick();
 }
 window.handleRoutingClick=function(){openUnified();};
 window.openRoutingModal=openUnified;window.openMtModal=openUnified;
 window.closeMtModal=function(){
  if(busy)return;
  if(mode==='xray'&&xrayDirty&&!confirm('Правила Xray ещё не сохранены. Закрыть редактор?'))return;
  if(mode==='xray'){
   xrayDirty=false;
   document.getElementById('mt_modal').style.display='none';
   return;
  }
  originalCloseMt();
 };
 window.closeRoutingModal=function(){closeMtModal();};
 window.switchRoutingRules=async function(engine){
  if(engine===mode||busy)return;
  if(mode==='xray'&&xrayDirty&&!confirm('Переключить движок? Несохранённые правила Xray будут потеряны.'))return;
  busy=true;refresh();_mtSetStatus('Переключение правил...','');
  try{
   var response=await fetch('/api/routing_rules_mode',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({engine:engine})});
   var result=await response.json();if(!response.ok)throw new Error(result.error||'Не удалось переключить правила');
   mode=result.engine;
   if(mode==='optirout'&&!MT_STATE.config)await originalOpenMt();
   if(mode==='xray')await loadXrayConfig();
  }catch(error){alert(error.message);}
  finally{busy=false;refresh();}
 };
 window.saveUnifiedRouting=async function(){
  if(busy)return;
  if(mode==='optirout'){if(MT_STATE.dirty)await saveMtConfig();return;}
  if(busy||!xrayDirty)return;
  var editor=document.getElementById('routing_modal');
  function value(name){var input=editor.querySelector('[name="'+name+'"]');return input?input.value:'';}
  var checked=editor.querySelector('[name="routing_mode"]:checked');
  var routing={enabled:editor.querySelector('[name="routing_enabled"]').checked,direct_default:checked&&checked.value==='direct_default',groups_ui:window.__rt_get_data?window.__rt_get_data():{}};
  ['routing_block_rules','routing_direct_rules','routing_proxy_rules'].forEach(function(name){routing[name.replace('routing_','')]=value(name);});
  if(window.xrayDuplicateEntries&&window.xrayDuplicateEntries().length){window.validateXrayGeoReferences();alert('В правилах есть дубли или совпадения между разделами. Уберите подсвеченные повторы перед сохранением.');return;}
  ['block','direct','proxy'].forEach(function(cat){if(window.xrayGeoRulesFirst)routing[cat+'_rules']=window.xrayGeoRulesFirst(routing[cat+'_rules']);});
  busy=true;refresh();_mtSetStatus('Сохранение правил Xray...','');
  try{
   var response=await fetch('/api/routing_rules_mode',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({engine:'xray',routing:routing})});
   var result=await response.json();if(!response.ok)throw new Error(result.error||'Не удалось сохранить правила');
   ['block','direct','proxy'].forEach(function(cat){editor.querySelector('[name="routing_'+cat+'_rules"]').value=routing[cat+'_rules'];});
   if(window.syncXraySplitFields)window.syncXraySplitFields();
   if(window.validateXrayGeoReferences)window.validateXrayGeoReferences();
   xrayBaseline=xraySignature();xrayDirty=false;
  }catch(error){alert(error.message);}
  finally{busy=false;refresh();}
 };
 document.addEventListener('DOMContentLoaded',function(){
  var link=document.querySelector('.subs-header-actions [data-routing-engine]');
  mode=link&&link.dataset.routingEngine==='xray'?'xray':'optirout';
  var modal=document.getElementById('mt_modal'),editor=document.getElementById('routing_modal');
  editor.className='unified-xray-editor';editor.removeAttribute('onclick');editor.removeAttribute('role');editor.hidden=true;
  var oldHeader=editor.querySelector('.modal-header');
  oldHeader.querySelector('.modal-title').remove();oldHeader.querySelector('.modal-close').remove();oldHeader.querySelector('#routing_save_btn').remove();
  modal.querySelector('.modal-content').insertBefore(editor,modal.querySelector('.mt-modal-footer'));
  var bar=document.createElement('div');bar.className='unified-routing-mode';
  bar.innerHTML='<div class="unified-rule-switch" role="group" aria-label="Движок правил маршрутизации"><button type="button" data-rule-mode="optirout" onclick="switchRoutingRules(this.dataset.ruleMode)">Правила OptiRout</button><button type="button" data-rule-mode="xray" onclick="switchRoutingRules(this.dataset.ruleMode)">Правила Xray</button></div>';
  var headerActions=modal.querySelector('.mt-sticky-top > .modal-header .routing-header-actions');
  headerActions.insertBefore(bar,headerActions.firstChild);
  var xrayIO=oldHeader.querySelector('.rt-io-group');
  xrayIO.classList.add('xray-header-io');
  xrayIO.querySelectorAll('.rt-import-export-btn').forEach(function(button){button.removeAttribute('style');});
  headerActions.insertBefore(xrayIO,bar.nextSibling);
  oldHeader.remove();
  var root=editor.querySelector('.rt-routing-root');
  var toolbar=document.createElement('div');toolbar.className='xr-toolbar mt-sticky-bar';
  var searchToolbar=document.createElement('div');searchToolbar.className='mt-search-toolbar';
  searchToolbar.innerHTML='<input type="search" id="xr_group_search" placeholder="Поиск групп и правил" aria-label="Поиск групп и правил">'+
   '<select id="xr_group_filter" aria-label="Фильтр групп"><option value="all">Все</option><option value="enabled">Включённые</option><option value="disabled">Выключенные</option></select><span class="xr-filter-count" id="xr_filter_count" aria-live="polite"></span>';
  toolbar.append(searchToolbar);
  root.before(toolbar);
  var enable=editor.querySelector('[name="routing_enabled"]').closest('.setting-field');
  enable.classList.add('xr-enable','mt-groups-toolbar');toolbar.prepend(enable);
  var reload=document.createElement('button');reload.type='button';reload.id='xr_reload_btn';reload.className='xr-reload-btn mt-reload-btn';
  reload.title='Перезагрузить конфиг из файла';reload.setAttribute('aria-label','Перезагрузить конфиг');
  reload.innerHTML='<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 7v5h-5M4 17v-5h5"/><path d="M6.2 6.2A8 8 0 0 1 20 12M4 12a8 8 0 0 0 13.8 5.8"/></svg>';
  reload.addEventListener('click',function(){window.reloadXrayConfig();});searchToolbar.append(reload);
  modal.querySelector('.mt-search-toolbar').append(modal.querySelector('.mt-reload-btn'));
  if(window.optirayEnhanceSelect)window.optirayEnhanceSelect(document.getElementById('xr_group_filter'));
  Array.from(root.children).forEach(function(child){
   var groups=child.querySelector('.rt-groups');
   if(groups){
    child.classList.add('xr-outbound');child.dataset.cat=groups.id.replace('rt_groups_','');
    var body=document.createElement('div');body.className='xr-outbound-body';
    Array.from(child.children).forEach(function(element){if(!element.classList.contains('setting-label'))body.append(element);});
    child.append(body);
    var heading=child.querySelector(':scope > .setting-label');
    if(heading){
     var toggle=document.createElement('button');toggle.type='button';toggle.className='xr-outbound-toggle';
     toggle.setAttribute('aria-label','Свернуть или развернуть аутбаунд');
     body.id='xr_outbound_body_'+child.dataset.cat;toggle.setAttribute('aria-controls',body.id);
     toggle.innerHTML='<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m9 5 7 7-7 7"/></svg>';
     var key='optiray_xray_outbound_collapsed_'+child.dataset.cat;
     function apply(collapsed){child.classList.toggle('xr-collapsed',collapsed);toggle.setAttribute('aria-expanded',String(!collapsed));}
     var saved=false;try{saved=localStorage.getItem(key)==='1';}catch(e){}apply(saved);
     toggle.addEventListener('click',function(event){
      event.preventDefault();event.stopPropagation();var collapsed=!child.classList.contains('xr-collapsed');
      apply(collapsed);try{localStorage.setItem(key,collapsed?'1':'0');}catch(e){}
     });
     heading.prepend(toggle);
    }
   }
  });
  var maintenance=document.createElement('details');maintenance.className='mt-maintenance xr-maintenance';
  maintenance.innerHTML='<summary>Обслуживание</summary>'+ 
   '<label style="display:flex;align-items:center;gap:8px;margin:16px 0 8px;cursor:pointer;color:var(--text);"><input type="checkbox" id="xr_backup_enabled" onchange="setXrayBackupEnabled(this)">Сохранять резервные копии конфигурации Xray</label>'+ 
   '<p>При включении сохраняются копии перед изменением конфигурации и правил. Хранятся последние 10 копий.</p>'+ 
   '<button type="button" class="btn mt-reset-btn" onclick="resetXrayRules()">Сбросить правила</button>'+ 
   '<p>Восстановление конфигурации маршрутизации Xray по умолчанию.</p>';
  root.append(maintenance);
  syncXrayOutboundOrder();
  var empty=document.createElement('div');empty.className='xr-empty';empty.hidden=true;empty.textContent='Группы не найдены';root.append(empty);
  window.filterXrayGroups=function(){
   var query=document.getElementById('xr_group_search').value.trim().toLowerCase();
   var filter=document.getElementById('xr_group_filter').value;
   var cards=editor.querySelectorAll('.rt-group-card'),visible=0;
   cards.forEach(function(card){
    var values=Array.from(card.querySelectorAll('.rt-group-title,.rt-rule-input')).map(function(input){return input.value;}).join(' ').toLowerCase();
    var matchedRule=false;
    card.querySelectorAll('.rt-group-title,.rt-rule-input').forEach(function(input){
     var match=!!query && input.value.toLowerCase().includes(query);
     input.classList.toggle('routing-search-match',match);
     if(match && input.classList.contains('rt-rule-input'))matchedRule=true;
    });
    if(query && !card.dataset.searchCollapseSaved){card.dataset.searchCollapseSaved='1';card.dataset.searchWasCollapsed=String(card.classList.contains('collapsed'));}
    if(query && matchedRule){card.classList.remove('collapsed');var section=card.closest('.xr-outbound');if(section)section.classList.remove('xr-collapsed');}
    if(!query && card.dataset.searchCollapseSaved){card.classList.toggle('collapsed',card.dataset.searchWasCollapsed==='true');delete card.dataset.searchCollapseSaved;delete card.dataset.searchWasCollapsed;}
    var enabled=card.querySelector('.rt-group-enabled').checked;
    card.hidden=(query&&values.indexOf(query)<0)||(filter==='enabled'&&!enabled)||(filter==='disabled'&&enabled)||(filter==='blocked'&&card.dataset.cat!=='block')||(['block','direct','proxy'].indexOf(filter)>=0&&card.dataset.cat!==filter);
    if(!card.hidden)visible++;
    var count=card.querySelector('.xr-group-count');
    if(count)count.textContent=ruCount(Array.from(card.querySelectorAll('.rt-rule-input')).filter(function(input){return input.value.trim();}).length,'правило','правила','правил');
   });
   editor.querySelectorAll('.xr-outbound').forEach(function(section){
    var categoryMismatch=['block','direct','proxy'].indexOf(filter)>=0&&section.dataset.cat!==filter;
    var matchingGroup=Array.from(section.querySelectorAll('.rt-group-card')).some(function(card){return !card.hidden;});
    var listMatches=query&&section.querySelector('textarea').value.toLowerCase().indexOf(query)>=0;
    section.hidden=categoryMismatch||((query||filter==='enabled'||filter==='disabled'||filter==='blocked')&&!matchingGroup&&!listMatches);
   });
   document.getElementById('xr_filter_count').textContent=visible+'/'+cards.length;
   if(mode==='xray')document.getElementById('mt_filter_count').textContent=visible+'/'+cards.length;
   empty.hidden=Array.from(editor.querySelectorAll('.xr-outbound')).some(function(section){return !section.hidden;});
  };
  ['input','change'].forEach(function(name){editor.addEventListener(name,function(event){
   if(event.optirayValidationOnly||event.target.id==='xr_backup_enabled')return;
   if(event.target.name==='routing_enabled'){if(name==='change')window.setRoutingPower(event.target,'xray');return;}
   window.filterXrayGroups();
   if(event.target.closest('.xr-toolbar')&&event.target.name!=='routing_enabled')return;
   xrayDirty=true;refresh();
  });});
  editor.addEventListener('click',function(event){
   if(event.target.closest('.rt-add-group,.rt-add-group-paste,.rt-add-rule,.rt-rule-del,.rt-group-del')){xrayDirty=true;refresh();}
  });
  window.filterXrayGroups();
  refresh();
 });
})();


function selectPasteMode(button, replace) {
 var toggle=button.closest('.paste-mode-toggle');
 var input=toggle.querySelector('input');input.checked=replace;
 toggle.querySelectorAll('button').forEach(function(item,index){
  item.setAttribute('aria-pressed',String(index===(replace?1:0)));
 });
 input.dispatchEvent(new Event('change',{bubbles:true}));
}

document.addEventListener('DOMContentLoaded',function(){
 var modal=document.getElementById('mt_modal');if(!modal)return;
 var header=modal.querySelector('.mt-sticky-top .routing-header-actions');
 var tools=modal.querySelector('.mt-groups-toolbar'),xrTools=modal.querySelector('.xr-toolbar');
 if(!header||!tools||!xrTools)return;
 var entries=Array.from(header.children).filter(function(el){return el.matches('.rt-io-group,.mt-io-backup');});
 entries.forEach(function(el){var marker=document.createComment('routing action position');el.before(marker);el.__mobilePosition=marker;});
 var media=window.matchMedia('(max-width:760px)');
 function arrange(){entries.forEach(function(el){if(media.matches){(el.classList.contains('xray-header-io')?xrTools:tools).appendChild(el);}else{el.__mobilePosition.after(el);}});}
 arrange();media.addEventListener('change',arrange);
});

document.addEventListener('DOMContentLoaded',function(){
 var media=window.matchMedia('(max-width:760px)');
 function arrangeModalScroll(){
  ['mt_modal','settings_modal'].forEach(function(id){
   var modal=document.getElementById(id);if(!modal)return;
   var content=modal.querySelector(':scope > .modal-content');if(!content)return;
   var wrapper=content.querySelector(':scope > .mobile-modal-scroll');
   if(media.matches&&!wrapper){
    var footer=content.querySelector(':scope > .mt-modal-footer,:scope > .modal-footer-btns');
    if(!footer)return;
    wrapper=document.createElement('div');wrapper.className='mobile-modal-scroll';
    var children=Array.from(content.children).filter(function(child){return child!==footer;});
    content.insertBefore(wrapper,footer);children.forEach(function(child){wrapper.appendChild(child);});
   }else if(!media.matches&&wrapper){while(wrapper.firstChild)content.insertBefore(wrapper.firstChild,wrapper);wrapper.remove();}
  });
 }
 arrangeModalScroll();media.addEventListener('change',arrangeModalScroll);
});

document.addEventListener('DOMContentLoaded',function(){
 var names=document.getElementById('subs_names_container'),row=document.querySelector('.subs-header-row');
 if(!names||!row)return;
 var chips=names.querySelectorAll('.subs-name-chip');
 chips.forEach(function(chip){chip.title=chip.textContent.trim();});
 var track=document.createElement('div');track.className='subs-ticker-track';
 chips.forEach(function(chip){track.appendChild(chip);});names.appendChild(track);
 names.classList.add('subs-ticker');
 var copies=Array.from(chips).map(function(chip){
  var copy=chip.cloneNode(true);copy.setAttribute('aria-hidden','true');
  copy.classList.add('subs-ticker-copy');track.appendChild(copy);return copy;
 });
 var distance=0,offset=0,paused=false,lastTime=0;
 function measureTicker(){
  var total=Array.from(chips).reduce(function(width,chip){return width+chip.getBoundingClientRect().width;},0);
  var style=getComputedStyle(names);
  var border=style.boxSizing==='border-box'?(parseFloat(style.borderLeftWidth)||0)+(parseFloat(style.borderRightWidth)||0):0;
  names.style.setProperty('--subs-content-width',(total+border)+'px');
  // clientWidth is rounded and excludes the border; a subpixel difference is not overflow.
  distance=total>names.clientWidth+1&&names.clientWidth>0&&!reducedMotion.matches?total:0;
  copies.forEach(function(copy){copy.hidden=!distance;});
  if(!distance){offset=0;track.style.transform='none';}
 }
 var tickerResize=new ResizeObserver(measureTicker);
 tickerResize.observe(names);
 chips.forEach(function(chip){tickerResize.observe(chip);});
 new MutationObserver(function(records){
  records.forEach(function(record){var index=Array.from(chips).indexOf(record.target);if(index>=0)copies[index].setAttribute('style',chips[index].getAttribute('style')||'');});
 }).observe(track,{attributes:true,subtree:true,attributeFilter:['style']});
 names.addEventListener('mouseenter',function(){paused=true;});
 names.addEventListener('mouseleave',function(){paused=false;});
 var reducedMotion=window.matchMedia('(prefers-reduced-motion: reduce)');
 reducedMotion.addEventListener('change',measureTicker);
 function tick(time){
  var elapsed=lastTime?Math.min(time-lastTime,100):0;lastTime=time;
  if(distance&&!paused&&!reducedMotion.matches&&names.offsetWidth){
   offset=(offset+elapsed*.025)%distance;
   track.style.transform='translateX(-'+offset+'px)';
  }
  requestAnimationFrame(tick);
 }
 requestAnimationFrame(tick);
 var marker=document.createComment('subscription labels original position');names.before(marker);
 var toggle=document.getElementById('subs_collapse_btn');
 if(!toggle)return;
 var group=document.createElement('div');group.className='subs-header-group';
 group.setAttribute('role','group');group.setAttribute('aria-label','Подписки');
 row.insertBefore(group,toggle);
 var media=window.matchMedia('(min-width:761px)');
 function arrange(){
  group.hidden=!media.matches;
  if(media.matches){group.append(toggle,names);}
  else{row.insertBefore(toggle,group);marker.after(names);}
 }
 arrange();media.addEventListener('change',arrange);
});

function ruCount(number,one,few,many){
 var n=Math.abs(Number(number)),last=n%10,lastTwo=n%100;
 return number+' '+(lastTwo>=11&&lastTwo<=14?many:last===1?one:last>=2&&last<=4?few:many);
}

document.addEventListener('DOMContentLoaded',function(){
 var panel=document.querySelector('.active-status-card'),name=document.getElementById('active-srv-name'),flag=document.getElementById('active-server-flag');
 if(!panel||!name||!flag)return;
 function clean(value){return value.replace(/^[\u{1F1E6}-\u{1F1FF}]{2}[\uFE0F\u200D]*\s*/u,'');}
 function update(){
  var raw=name.textContent.trim(),label=clean(raw);
  var card=Array.from(document.querySelectorAll('#servers_container .server-card')).find(function(el){return el.dataset.name===raw||clean(el.dataset.name||'')===label;});
  if(card){
   var section=card.closest('.sub-section'),image=card.querySelector('.srv-flag');
   if(section&&/^#[0-9a-f]{6}$/i.test(section.dataset.subcolor||''))panel.style.setProperty('--active-subscription-color',section.dataset.subcolor);
   flag.replaceChildren();if(image)flag.appendChild(image.cloneNode(true));
  }
  if(raw!==label)name.textContent=label;
 }
 update();new MutationObserver(update).observe(name,{childList:true,characterData:true,subtree:true});
});

async function toggleOptirayPower(toggle){
 if(toggle.disabled)return;var wanted=toggle.checked,status=document.getElementById('optiray_power_status');toggle.disabled=true;syncPowerButtons();var label=document.getElementById('optiray_power_label');if(label)label.textContent=wanted?'OptiRay вкл.':'OptiRay выкл.';if(status)status.textContent=wanted?'Включение…':'Отключение…';
 try{var response=await fetch('/api/optiray_enabled',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({enabled:wanted})});var data=await response.json();if(!response.ok||!data.ok)throw new Error(data.error||'Ошибка переключения');if(status)status.textContent=wanted?'OptiRay включён':'OptiRay выключен';await new Promise(function(resolve){setTimeout(resolve,3000)});window.location.reload();}
 catch(error){toggle.checked=!wanted;if(label)label.textContent=toggle.checked?'OptiRay вкл.':'OptiRay выкл.';if(status)status.textContent=error.message;await new Promise(function(resolve){setTimeout(resolve,3000)});toggle.disabled=false;syncPowerButtons();}
}

function mtUpdatePowerUi(){
 var enabled=!MT_STATE.config||MT_STATE.config.enabled!==false;
 var toggle=document.getElementById('mt_enabled'),label=document.getElementById('mt_power_label'),groups=document.getElementById('mt_groups_container');
 if(toggle)toggle.checked=enabled;if(label)label.textContent=enabled?'OptiRout вкл.':'OptiRout выкл.';
 if(groups)groups.classList.toggle('optirout-disabled',!enabled);syncPowerButtons();
}
function mtPowerChanged(toggle){if(!MT_STATE.config)return;window.setRoutingPower(toggle,'optirout');}

function syncPowerButtons(){
 document.querySelectorAll('[data-power-target]').forEach(function(button){
  var input=document.getElementById(button.dataset.powerTarget);if(!input)return;
  var name=button.dataset.powerTarget==='optiray_power'?'OptiRay':button.dataset.powerTarget==='mt_enabled'?'OptiRout':'Xray Routing';
  button.setAttribute('aria-pressed',String(input.checked));button.disabled=input.disabled;if(button.classList.contains('routing-title-power'))button.title=name+': '+(input.checked?'Включён':'Выключен');
  button.querySelector('span').textContent=button.classList.contains('routing-title-power')?name:name+(input.checked?' вкл.':' выкл.');
  button.setAttribute('aria-label',(input.checked?'Выключить ':'Включить ')+name);
 });
}
function powerButtonClick(button){
 var input=document.getElementById(button.dataset.powerTarget);if(!input||button.disabled)return;
 input.checked=!input.checked;syncPowerButtons();
 if(input.id==='optiray_power'){toggleOptirayPower(input);}
 else if(input.id==='mt_enabled'){mtPowerChanged(input);}
 else{input.dispatchEvent(new Event('change',{bubbles:true}));}
}
if(document.readyState==='loading'){document.addEventListener('DOMContentLoaded',syncPowerButtons);}else{syncPowerButtons();}

function subscriptionDisclosureIcon() {
    return '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="2" y="4" width="12" height="16" rx="2"/><path d="M5 8h6M5 12h6M5 16h3"/><path class="subs-disclosure-chevron" d="m17 9 3 3-3 3"/></svg>';
}
// Visible manual color controls avoid clipped native picker anchors.
document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('#subs_container .sub-row').forEach(function (row) {
        var auto = row.querySelector('.dice-btn');
        if (!auto) return;
        auto.title = 'Автоцвет';
        auto.setAttribute('aria-label', 'Выбрать автоматический цвет');
        auto.onclick = function () { randomSubColor(Number(row.dataset.subId)); };
        var button = document.createElement('button');
        button.type = 'button';
        button.className = 'sub-manual-color-trigger';
        row.appendChild(button);
        button.removeAttribute('onclick');
        button.title = 'Выбрать цвет';
        button.setAttribute('aria-label', 'Выбрать цвет подписки');
        button.addEventListener('click', function (event) {
            event.stopPropagation();
            var old = document.getElementById('sub_manual_palette');
            if (old) { if (old.closePalette) old.closePalette(); else old.remove(); }
            var row = button.closest('.sub-row');
            var source = row.querySelector('input[type="color"]');
            if (!source) return;
            var panel = document.createElement('div');
            panel.id = 'sub_manual_palette';
            panel.setAttribute('role', 'dialog');
            panel.setAttribute('aria-label', 'Цвет подписки');
            var picker = document.createElement('input');
            picker.type = 'color'; picker.value = source.value;
            picker.setAttribute('aria-label', 'Выбрать цвет');
            var hex = document.createElement('input');
            hex.type = 'text'; hex.value = source.value; hex.maxLength = 7;
            hex.setAttribute('aria-label', 'HEX-код цвета');
            async function apply(value) {
                if (!/^#[0-9a-f]{6}$/i.test(value)) return;
                
                var saved = await changeSubColor(Number(row.dataset.subId), value);
                picker.value = source.value; hex.value = source.value;
            }
            picker.addEventListener('change', function () { apply(picker.value); });
            hex.addEventListener('change', function () { apply(hex.value.trim()); });
            panel.append(picker, hex);
            panel.addEventListener('click', function (e) { e.stopPropagation(); });
            document.body.appendChild(panel);
            var rect = button.getBoundingClientRect();
            panel.style.left = Math.max(8, Math.min(rect.right - 180, innerWidth - 188)) + 'px';
            panel.style.top = Math.min(rect.bottom + 5, innerHeight - 70) + 'px';
            function close(e) {
                if (e.type === 'keydown' && e.key !== 'Escape') return;
                panel.remove();
                document.removeEventListener('click', close);
                document.removeEventListener('keydown', close);
            }
            panel.closePalette = function () { close({type:'click'}); };
            document.addEventListener('click', close);
            document.addEventListener('keydown', close);
        });
    });
});
/* Automatic server selection uses a wand icon. */
document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('#servers_container .sub-actions .btn-auto-select').forEach(function (button) {
        if (button.querySelector('.auto-select-icon')) return;
        button.insertAdjacentHTML('afterbegin', '<svg class="action-icon auto-select-icon" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="7"/><circle cx="12" cy="12" r="2.5"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3"/></svg>');
    });
});

// Engine icons for the header mode switch.
document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('#mt_modal .unified-rule-switch > button').forEach(function (button) {
        var path = button.dataset.ruleMode === 'optirout'
            ? '<rect x="3" y="3" width="5" height="5" rx="1"/><rect x="16" y="3" width="5" height="5" rx="1"/><rect x="9.5" y="16" width="5" height="5" rx="1"/><path d="M5.5 8v3.5h13V8M12 11.5V16"/>'
            : '<path d="m13 2-9 12h7l-1 8 10-12h-7Z"/>';
        button.insertAdjacentHTML('afterbegin','<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">'+path+'</svg>');
    });
});

// Files entering/leaving the panel and a restore history icon.
document.addEventListener('DOMContentLoaded', function () {
    var icons = {
        '.mt-io-import': ['Загрузить правила из файла Excel', '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6Z"/><path d="M14 2v6h6M12 11v7m-3-3 3 3 3-3"/>'],
        '.mt-io-export': ['Скачать правила в файл Excel', '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6Z"/><path d="M14 2v6h6M12 18v-7m-3 3 3-3 3 3"/>'],
        '.mt-io-backup': ['Восстановить правила из резервной копии', '<path d="M3 10a9 9 0 1 1 2 8M3 4v6h6"/><path d="M12 7v5l3 2"/>']
    };
    Object.keys(icons).forEach(function (selector) {
        document.querySelectorAll('#mt_modal .routing-header-actions ' + selector).forEach(function (button) {
            button.title=icons[selector][0];button.setAttribute('aria-label',button.title);
            button.innerHTML='<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">'+icons[selector][1]+'</svg>';
        });
    });
});

// Labeled routing toolbar actions.
document.addEventListener('DOMContentLoaded', function () {
    var actions = {
        '.mt-add-group':['Добавить','<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M12 8v8M8 12h8"/>'],
        '.mt-global-iface-btn':['Интерфейс','<rect x="3" y="3" width="5" height="5" rx="1"/><rect x="16" y="3" width="5" height="5" rx="1"/><rect x="9.5" y="16" width="5" height="5" rx="1"/><path d="M5.5 8v3h13V8M12 11v5"/>'],
        '.mt-paste-group-btn':['Вставить','<rect x="5" y="5" width="14" height="16" rx="2"/><rect x="9" y="3" width="6" height="4" rx="1"/><path d="M9 12h6M9 16h6"/>'],
        '.mt-reload-btn':['Обновить','<path d="M20 11a8 8 0 0 0-14-5L3 9m0-5v5h5M4 13a8 8 0 0 0 14 5l3-3m0 5v-5h-5"/>']
    };
    Object.keys(actions).forEach(function(selector){
        document.querySelectorAll('#mt_modal .mt-sticky-bar '+selector).forEach(function(button){
            button.innerHTML='<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">'+actions[selector][1]+'</svg><span>'+actions[selector][0]+'</span>';
        });
    });
});

// Place clipboard import directly after adding a routing group.
document.addEventListener('DOMContentLoaded', function () {
    var add=document.querySelector('#mt_modal .mt-groups-toolbar .mt-add-group');
    var paste=document.querySelector('#mt_modal .mt-groups-toolbar .mt-paste-group-btn');
    if(add && paste){
        add.after(paste);
        var label=paste.querySelector('span');
        if(label)label.textContent='Из буфера';
    }
});

document.addEventListener('DOMContentLoaded', function () {
    var button=document.querySelector('#mt_modal .mt-global-iface-btn');
    if(button){var icon=button.querySelector('svg');if(icon)icon.innerHTML='<path d="M5 4h14v12l-3 4H8l-3-4Z"/><path d="M8 4v5M11 4v5M14 4v5M17 4v5M9 20v2M15 20v2"/>';}
});

// Add a subscription beside its disclosure control, retaining collapse visibility.
document.addEventListener('DOMContentLoaded', function () {
    var row=document.getElementById('subs_add_row');
    var group=document.querySelector('.subs-header-group');
    if(!row || !group)return;
    group.after(row);
    var button=row.querySelector('button');
    button.innerHTML='<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M12 8v8M8 12h8"/></svg><span>Добавить подключение</span>';
    var subscriptions=document.getElementById('subs_container');
    row.style.display=subscriptions && subscriptions.style.display==='none'?'none':'';
});

// Keep the add action inside the same compact group as the disclosure.
document.addEventListener('DOMContentLoaded', function () {
    var toggle=document.getElementById('subs_collapse_btn');
    var add=document.getElementById('subs_add_row');
    if(toggle && add)toggle.after(add);
});

document.addEventListener('DOMContentLoaded', function () {
    var actions=document.querySelector('#mt_modal .routing-header-actions');
    var toggle=actions && actions.querySelector('.unified-routing-mode');
    if(actions && toggle){
        Array.from(actions.children).filter(function(el){return el.classList.contains('rt-io-group') || el.classList.contains('xray-header-io');}).forEach(function(el){actions.insertBefore(el,toggle);});
    }
    var close=document.querySelector('#mt_modal .mt-sticky-top > .modal-header > button.modal-close');
    if(close)close.innerHTML='<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg>';
});

document.addEventListener('DOMContentLoaded', function () {
    var actions=document.querySelector('#mt_modal .routing-header-actions');
    var toggle=actions && actions.querySelector('.unified-routing-mode');
    if(!toggle)return;
    Array.from(actions.children).filter(function(el){return el.classList.contains('rt-io-group');}).forEach(function(button){
        actions.insertBefore(button,toggle);
    });
});

function syncSharedXraySearch(){
 var search=document.getElementById('xr_group_search'),filter=document.getElementById('xr_group_filter');
 if(!search||!filter)return;
 search.value=document.getElementById('mt_group_search').value;
 var selectedFilter=document.getElementById('mt_group_filter').value;
 if(selectedFilter==='blocked')selectedFilter='all';
 filter.value=selectedFilter;
 if(window.filterXrayGroups)window.filterXrayGroups();
}
function syncRoutingHeaderActions(mode){
 var sharedFilter=document.getElementById('mt_group_filter');
 if(sharedFilter){
  var blockedOption=sharedFilter.querySelector('option[value=blocked]');
  if(mode==='xray' && blockedOption){
   if(sharedFilter.value==='blocked')sharedFilter.value='all';
   blockedOption.remove();
   sharedFilter.dispatchEvent(new Event('change',{bubbles:true}));
  }else if(mode!=='xray' && !blockedOption){
   var option=document.createElement('option');option.value='blocked';option.textContent='Заблокированные';sharedFilter.appendChild(option);
  }
 }
 var toolbar=document.querySelector('#mt_modal .mt-groups-toolbar');if(!toolbar)return;
 var add=toolbar.querySelector('.mt-add-group'),paste=toolbar.querySelector('.mt-paste-group-btn'),iface=toolbar.querySelector('.mt-global-iface-btn'),reload=document.querySelector('#mt_modal .mt-sticky-bar .mt-reload-btn');
 if(!add.dataset.originalClick){add.dataset.originalClick=add.getAttribute('onclick');paste.dataset.originalClick=paste.getAttribute('onclick');reload.dataset.originalClick=reload.getAttribute('onclick');}
 if(mode==='xray'){
  add.onclick=function(){document.querySelector('#routing_modal .rt-add-group')?.click();};
  paste.onclick=function(){document.querySelector('#routing_modal .rt-add-group-paste')?.click();};
  reload.onclick=function(){window.reloadXrayConfig();};
  iface.style.setProperty('display','none','important');
  syncSharedXraySearch();
 }else{
  add.setAttribute('onclick',add.dataset.originalClick);paste.setAttribute('onclick',paste.dataset.originalClick);reload.setAttribute('onclick',reload.dataset.originalClick);
  iface.style.removeProperty('display');
 }
}
document.addEventListener('DOMContentLoaded', function () {
    var icons={
        '.btn-pingfav':'<path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2-5.6-3-5.6 3 1.1-6.2L3 9.6l6.2-.9Z"/>',
        '.btn-pingtcp':'<rect x="2" y="8" width="5" height="8" rx="1"/><rect x="17" y="8" width="5" height="8" rx="1"/><path d="M7 10h10M7 14h10M4 5v3M20 5v3"/>',
        '.btn-pingall':'<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c5 5 5 13 0 18-5-5-5-13 0-18Z"/>',
        '.btn-stop':'<rect x="5" y="5" width="14" height="14" rx="2"/>'
    };
    Object.keys(icons).forEach(function(selector){var svg=document.querySelector('#global_ping_row '+selector+' svg');if(svg){svg.setAttribute('stroke-width','1.7');svg.innerHTML=icons[selector];}});
});

function showMaintenanceWait(mode,title,status){
 var modal=document.getElementById('wait_modal');
 var animation=modal.querySelector('.update-spinner');
 if(!animation.dataset.originalMarkup)animation.dataset.originalMarkup=animation.innerHTML;
 modal.dataset.operation=mode;
 animation.classList.add('maintenance-animation');
 animation.innerHTML='<svg viewBox="0 0 160 80" fill="none"><g class="maintenance-file"><path d="M20 14h25l12 12v35H20Z"/><path d="M45 14v12h12M28 34h20M28 42h20M28 50h12"/></g><path class="maintenance-link" d="M64 40h28"/><circle class="maintenance-data" cx="65" cy="40" r="3"/><g class="maintenance-router"><rect x="98" y="34" width="46" height="28" rx="7"/><path d="M105 34V19M137 34V19M108 44h26"/><circle cx="110" cy="53" r="1.5"/><circle cx="117" cy="53" r="1.5"/></g><path class="maintenance-reset" d="M119 14a14 14 0 1 1-11 22M104 23v13h13"/></svg>';
 showWaitModal(title,status);
}
function setXraySubscriptionAvailability(installed){
 window.optirayXrayInstalled=!!installed;
 document.querySelectorAll('#subs_add_row button,.btn-refresh-icon,button[name="action"][value="update"]').forEach(function(button){button.disabled=!installed;if(!installed)button.title='Установите ядро Xray в настройках';});
 var card=document.querySelector('.active-status-card');
 if(card)card.classList.toggle('xray-missing',!installed);
 var notice=document.querySelector('.xray-install-notice');if(notice)notice.hidden=installed;
}
document.addEventListener('DOMContentLoaded',function(){
 setXraySubscriptionAvailability(document.getElementById('xray_available')?.value==='true');
});
document.addEventListener('DOMContentLoaded',function(){
    var icons=document.querySelectorAll('#settings_modal .settings-backup-icon');
    var paths=[
        '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6Z"/><path d="M14 2v6h6M12 11v7m-3-3 3 3 3-3"/>',
        '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6Z"/><path d="M14 2v6h6M12 18v-7m-3 3 3-3 3 3"/>'
    ];
    icons.forEach(function(icon,index){if(paths[index])icon.innerHTML='<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">'+paths[index]+'</svg>';});
});

document.addEventListener('click',function(event){
 var button=event.target.closest('.config-edit-link');if(!button)return;
 var modal=document.getElementById('config_edit_modal');
 if(!modal){modal=document.createElement('div');modal.id='config_edit_modal';modal.className='modal-overlay';modal.innerHTML='<div class="modal-content" style="width:600px;max-width:calc(100vw - 32px)"><div class="modal-header"><span class="config-editor-heading">Редактирование конфигурации <span class="config-editor-name"></span></span><button type="button" class="modal-close" aria-label="Закрыть"><svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" aria-hidden="true"><path d="m6 6 12 12M6 18 18 6"/></svg></button></div><div style="padding:16px 24px"><textarea class="setting-input" rows="6" style="width:100%;box-sizing:border-box;resize:vertical" aria-label="Ссылка конфигурации"></textarea><div class="config-edit-error" role="alert" style="color:var(--danger);margin-top:8px"></div></div><div class="modal-footer-btns"><button type="button" class="btn config-delete-btn">Удалить</button><button type="button" class="btn modal-footer-cancel">Отмена</button><button type="button" class="btn core-btn core-btn-install">Сохранить</button></div></div>';document.body.appendChild(modal);
 var close=function(){modal.style.display='none';};modal.querySelector('.modal-close').onclick=close;modal.querySelector('.modal-footer-cancel').onclick=close;modal.addEventListener('click',function(e){if(e.target===modal)close();});modal.addEventListener('keydown',function(e){if(e.key==='Escape')close();});
 modal.querySelector('.core-btn-install').onclick=async function(){var save=this;save.disabled=true;try{var body=new URLSearchParams({action:'edit_config',sub_id:modal.dataset.subId,config_index:modal.dataset.index,config_link:modal.querySelector('textarea').value});var response=await fetch('/manage',{method:'POST',body:body});if(!response.ok){var data=await response.json();throw Error(data.error||'Не удалось сохранить');}location.reload();}catch(error){modal.querySelector('.config-edit-error').textContent=error.message;}finally{save.disabled=false;}};
 }
 modal.querySelector('.config-editor-name').textContent=button.textContent.trim();
 modal.querySelector('.config-delete-btn').onclick=async function(){
  if(!confirm('Удалить конфигурацию «'+button.textContent.trim()+'»?'))return;
  this.disabled=true;
  var finishDeletion=showConfigDeletion(button.textContent.trim());
  try{var response=await fetch('/manage',{method:'POST',body:new URLSearchParams({action:'delete_config',sub_id:modal.dataset.subId,config_index:modal.dataset.index})});if(!response.ok){var data=await response.json();throw Error(data.error||'Не удалось удалить');}location.reload();}catch(error){finishDeletion();this.disabled=false;modal.querySelector('.config-edit-error').textContent=error.message;}
 };
 modal.dataset.subId=button.dataset.subId;modal.dataset.index=button.dataset.configIndex;modal.querySelector('textarea').value=button.dataset.configLink;modal.querySelector('.config-edit-error').textContent='';modal.style.display='flex';modal.querySelector('textarea').focus();
});
function showConfigDeletion(name,title){
 var overlay=document.createElement('div');overlay.className='modal-overlay config-deletion-overlay';overlay.style.display='flex';overlay.tabIndex=-1;
 overlay.innerHTML='<div class="config-deletion-card" role="alertdialog" aria-modal="true" aria-labelledby="config_deletion_title" aria-busy="true"><svg class="config-deletion-icon" viewBox="0 0 64 64" width="64" height="64" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle class="config-deletion-orbit" cx="32" cy="32" r="28" stroke-dasharray="70 106"/><g class="config-deletion-bin"><path d="M23 25h18l-2 23H25l-2-23Zm-3 0h24M28 20h8l2 5M29 31v11m6-11v11"/></g></svg><div id="config_deletion_title">Удаление конфигурации</div><div class="config-deletion-name"></div><div class="config-deletion-hint" role="status">Подождите, удаление выполняется…</div></div>';
 overlay.querySelector('.config-deletion-name').textContent=name;
 if(title)overlay.querySelector('#config_deletion_title').textContent=title;
 var previous=document.activeElement,locked=[];
 Array.from(document.body.children).forEach(function(element){if(!element.inert){element.inert=true;locked.push(element);}});
 document.body.appendChild(overlay);overlay.focus();
 function block(event){if(event.key==='Escape'||event.key==='Tab'){event.preventDefault();event.stopImmediatePropagation();}}
 document.addEventListener('keydown',block,true);
 return function(){document.removeEventListener('keydown',block,true);overlay.remove();locked.forEach(function(element){element.inert=false;});if(previous&&previous.isConnected)previous.focus();};
}
// The native confirmation runs first; keep the page locked while deleting the source.
document.addEventListener('submit',async function(event){
 var button=event.submitter;
 if(!button || button.name!=='action' || !/^del_\d+$/.test(button.value) || !button.closest('#subs_container'))return;
 event.preventDefault();
 var form=event.target;if(form.dataset.deletionBusy==='1')return;
 var row=button.closest('.sub-row');
 var name=row.querySelector('.sub-title-badge')?.textContent.trim()||'Подписка';
 var configurations=!!row.querySelector('.config-edit-link')||name==='Конфигурации';
 var body=new URLSearchParams();new FormData(form).forEach(function(value,key){if(typeof value==='string')body.append(key,value);});body.set('action',button.value);
 form.dataset.deletionBusy='1';
 var finish=showConfigDeletion(name,configurations?'Удаление всех конфигураций':'Удаление подписки');
 try{
  var response=await fetch(form.getAttribute('action')||'/manage',{method:'POST',body:body});
  if(!response.ok){var data=await response.json();throw Error(data.error||'Не удалось удалить');}
  location.reload();
 }catch(error){finish();delete form.dataset.deletionBusy;alert('Ошибка удаления: '+error.message);}
},true);
// A single busy policy covers configuration writes and existing operation dialogs.
(function(){
 var originalFetch=window.fetch.bind(window), pending=0, finishBusy=null;
 var mutations=new Set(['/manage','/select','/api/auto_select','/api/restore','/api/reset_defaults','/api/xray_action','/api/hysteria_action','/api/optirout_import','/api/optirout_config','/api/optirout_reset','/api/optirout_restore_backup','/api/routing_import','/api/xray_reset','/api/routing_rules_mode','/api/routing_power','/api/optiray_enabled','/api/reorder_subs','/api/save_sub_color','/api/optirout_backup_settings','/api/xray_backup_settings']);
 function operationVisible(){return document.querySelector('.config-deletion-overlay,#wait_modal.active,#update_overlay.active');}
 window.fetch=async function(input,options){
  var path;try{path=new URL(typeof input==='string'?input:input.url,location.href).pathname;}catch(_){return originalFetch(input,options);}
  var method=((options&&options.method)||(input&&input.method)||'GET').toUpperCase();
  if(method==='GET'||!mutations.has(path))return originalFetch(input,options);
  if(!pending&&!operationVisible()){
   finishBusy=showConfigDeletion('',path==='/select'?'Применение конфигурации':'Сохранение изменений');
   var overlay=document.querySelector('.config-deletion-overlay');overlay.classList.add('config-write-overlay');
   overlay.querySelector('.config-deletion-hint').textContent='Подождите, операция выполняется…';
   overlay.querySelector('.config-deletion-bin').innerHTML='<path d="m23 33 6 6 13-14"/>';
  }
  pending++;
  try{var response=await originalFetch(input,options);await response.clone().arrayBuffer();return response;}
  finally{pending--;if(!pending&&finishBusy){finishBusy();finishBusy=null;}}
 };
 var locked=[],active=null;
 function sync(){
  var next=operationVisible();if(next===active)return;
  locked.forEach(function(el){el.inert=false;});locked=[];active=next;
  if(!next)return;
  Array.from(document.body.children).forEach(function(el){if(el!==next&&!el.contains(next)&&!el.inert){el.inert=true;locked.push(el);}});
 }
 new MutationObserver(sync).observe(document.body,{subtree:true,childList:true,attributes:true,attributeFilter:['class','style']});
 document.addEventListener('keydown',function(e){if(operationVisible()&&(e.key==='Escape'||e.key==='Enter'||e.key===' ')){e.preventDefault();e.stopImmediatePropagation();}},true);
 // Standard form navigation also needs immediate feedback, before the next page arrives.
 document.addEventListener('submit',function(e){
  if(e.defaultPrevented||e.target.getAttribute('action')!=='/manage')return;
  if(operationVisible()){e.preventDefault();return;}
  var value=e.submitter&&e.submitter.value;
  showConfigDeletion('',value==='add'?'Добавление подключения':'Сохранение настроек');
  var overlay=document.querySelector('.config-deletion-overlay');overlay.classList.add('config-write-overlay');overlay.querySelector('.config-deletion-hint').textContent='Подождите, операция выполняется…';overlay.querySelector('.config-deletion-bin').innerHTML='<path d="m23 33 6 6 13-14"/>';
 });
 window.addEventListener('pageshow',function(e){if(e.persisted)location.reload();});
})();
function validRouterIPv4(value,partial){
 if(!/^[0-9.]*$/.test(value)||value.length>15)return false;
 var parts=value.split('.');if(parts.length>4||(!partial&&parts.length!==4))return false;
 return parts.every(function(part,index){return (partial&&index===parts.length-1&&part==='')||(/^\d{1,3}$/.test(part)&&Number(part)<=255);});
}
document.addEventListener('DOMContentLoaded',function(){
 var field=document.querySelector('#settings_modal input[name="proxy_host"]');if(!field)return;
 field.inputMode='decimal';field.maxLength=15;
 var previous=validRouterIPv4(field.value,true)?field.value:'';
 function validate(){field.setCustomValidity(validRouterIPv4(field.value,false)?'':'Введите IPv4: четыре числа от 0 до 255, например 192.168.1.1');}
 field.addEventListener('beforeinput',function(event){
  if(!event.inputType.startsWith('insert')||event.data==null)return;
  var next=field.value.slice(0,field.selectionStart)+event.data+field.value.slice(field.selectionEnd);
  if(!validRouterIPv4(next,true))event.preventDefault();
 });
 field.addEventListener('paste',function(event){
  var text=event.clipboardData.getData('text').trim();
  var next=field.value.slice(0,field.selectionStart)+text+field.value.slice(field.selectionEnd);
  if(!validRouterIPv4(next,true)){event.preventDefault();}
 });
 field.addEventListener('input',function(){
  if(!validRouterIPv4(field.value,true)){var pos=field.selectionStart;field.value=previous;field.setSelectionRange(Math.max(0,pos-1),Math.max(0,pos-1));}
  else previous=field.value;
  validate();
 });
 field.addEventListener('blur',validate);
});
document.addEventListener('DOMContentLoaded',function(){
 var field=document.querySelector('#settings_modal input[name="proxy_port"]');if(!field)return;
 field.min='1';field.max='65535';field.step='1';field.inputMode='numeric';
 var previous=field.value;
 function partial(value){return value===''||(/^\d{1,5}$/.test(value)&&Number(value)<=65535);}
 function validate(){field.setCustomValidity(/^\d{1,5}$/.test(field.value)&&Number(field.value)>=1&&Number(field.value)<=65535?'':'Введите порт от 1 до 65535');}
 field.addEventListener('keydown',function(e){if(['e','E','+','-','.',','].includes(e.key))e.preventDefault();});
 field.addEventListener('paste',function(e){var value=e.clipboardData.getData('text').trim();if(!/^\d{1,5}$/.test(value)||Number(value)<1||Number(value)>65535)e.preventDefault();});
 field.addEventListener('input',function(){if(!partial(field.value))field.value=previous;else previous=field.value;validate();});
 field.addEventListener('blur',validate);
});
document.addEventListener('DOMContentLoaded',function(){
 var field=document.getElementById('ping_timeout_input');if(!field)return;
 field.min='0.5';field.max='20';field.step='0.5';field.inputMode='decimal';
 var previous=field.value;
 function allowed(value){return value===''||(/^\d{1,2}(?:\.\d*)?$/.test(value)&&Number(value)<=20);}
 function validate(){var value=Number(field.value);field.setCustomValidity(field.value!==''&&Number.isFinite(value)&&value>=0.5&&value<=20?'':'Введите таймаут от 0,5 до 20 секунд');}
 field.addEventListener('keydown',function(event){if(['-','+','e','E'].includes(event.key))event.preventDefault();});
 field.addEventListener('paste',function(event){var text=event.clipboardData.getData('text').trim();if(!allowed(text))event.preventDefault();});
 field.addEventListener('input',function(){if(!allowed(field.value))field.value=previous;else previous=field.value;validate();});
 field.addEventListener('blur',validate);
});
// Connected status doubles as the disconnect control, including newly rendered cards.
(function(){
 function enhance(root){
  if(!root.querySelectorAll)return;
  var items=Array.from(root.querySelectorAll('.badge-connected:not(button)'));
  if(root.matches&&root.matches('.badge-connected:not(button)'))items.unshift(root);
  items.forEach(function(status){var button=document.createElement('button');button.type='button';button.className=status.className+' disconnect-optiray';button.setAttribute('aria-label','Отключить OptiRay');button.title='Отключить OptiRay';button.innerHTML='<span class="connected-caption">Подключено</span><span class="disconnect-caption">Отключить</span>';status.replaceWith(button);});
 }
 document.addEventListener('DOMContentLoaded',function(){var root=document.getElementById('servers_container');if(!root)return;enhance(root);new MutationObserver(function(records){records.forEach(function(record){record.addedNodes.forEach(enhance);});}).observe(root,{childList:true,subtree:true});});
 document.addEventListener('click',async function(event){
  var button=event.target.closest('.disconnect-optiray');if(!button)return;
  event.preventDefault();event.stopPropagation();var toggle=document.getElementById('optiray_power');if(!toggle||toggle.disabled||button.disabled)return;
  button.disabled=true;toggle.checked=false;
  var finish=showConfigDeletion('','Отключение OptiRay');
  var overlay=document.querySelector('.config-deletion-overlay');overlay.classList.add('config-write-overlay');overlay.querySelector('.config-deletion-hint').textContent='Остановка подключения…';overlay.classList.add('disconnect-operation');overlay.querySelector('.config-deletion-icon').outerHTML='<svg class="disconnect-link-animation" viewBox="0 0 160 80" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><g class="disconnect-router"><rect x="8" y="30" width="36" height="26" rx="6"/><path d="M15 30V18m22 12V18M17 44h1m6 0h1m6 0h5"/></g><g class="disconnect-server"><rect x="116" y="20" width="35" height="40" rx="6"/><path d="M123 31h20m-20 10h20m-20 10h10"/><circle cx="141" cy="51" r="1"/></g><g class="disconnect-left-wire"><path d="M49 40h25m0-4v8"/></g><g class="disconnect-right-wire"><path d="M86 40h25m-25-4v8"/></g><circle class="disconnect-packet packet-a" cx="56" cy="40" r="2.5" fill="currentColor" stroke="none"/><circle class="disconnect-packet packet-b" cx="68" cy="40" r="2.5" fill="currentColor" stroke="none"/><circle class="disconnect-packet packet-c" cx="99" cy="40" r="2.5" fill="currentColor" stroke="none"/></svg>';
  try{await toggleOptirayPower(toggle);if(toggle.checked){finish();button.disabled=false;var status=document.getElementById('optiray_power_status');alert(status?status.textContent:'Не удалось отключить OptiRay');}}
  catch(error){finish();button.disabled=false;alert(error.message);}
 });
})();
// Display connection state instead of the remembered server when the core is stopped.
document.addEventListener('DOMContentLoaded',function(){
 var panel=document.querySelector('.active-status-card'),dot=document.getElementById('active-status-dot');if(!panel||!dot)return;
 var text=panel.querySelector('.active-status-text');if(!text)return;
 var message=document.createElement('div');message.className='connection-absent-message';message.textContent='Соединение отсутствует';text.appendChild(message);
 function sync(){var disconnected=!dot.classList.contains('connected');panel.classList.toggle('connection-absent',disconnected);if(disconnected)panel.style.setProperty('--active-subscription-color','#6c757d');}
 sync();new MutationObserver(sync).observe(dot,{attributes:true,attributeFilter:['class']});
});
(function(){
 var menu,anchor,dialog;
 function closeMenu(){if(menu)menu.remove();menu=null;if(anchor)anchor.setAttribute('aria-expanded','false');}
 function enhance(root){if(!root.querySelectorAll)return;var cards=Array.from(root.querySelectorAll('.server-card'));if(root.matches&&root.matches('.server-card'))cards.unshift(root);cards.forEach(function(card){if(card.querySelector('.server-menu-trigger'))return;var button=document.createElement('button');button.type='button';button.className='server-menu-trigger';button.setAttribute('aria-label','Действия с сервером');button.setAttribute('aria-haspopup','menu');button.setAttribute('aria-expanded','false');button.innerHTML='<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor" aria-hidden="true"><circle cx="12" cy="5" r="1.6"/><circle cx="12" cy="12" r="1.6"/><circle cx="12" cy="19" r="1.6"/></svg>';card.appendChild(button);});}
 async function viewConfig(card){
  closeMenu();
  if(!dialog){dialog=document.createElement('div');dialog.className='modal-overlay';dialog.id='server_config_viewer';dialog.innerHTML='<div class="modal-content"><div class="modal-header"><span>Просмотр конфига <span class="server-config-name"></span></span><button type="button" class="modal-close" aria-label="Закрыть"><svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="m6 6 12 12M6 18 18 6"/></svg></button></div><div class="server-config-body"><textarea readonly spellcheck="false" aria-label="Конфигурация сервера"></textarea><div class="server-config-error" role="alert"></div></div></div>';document.body.appendChild(dialog);dialog.querySelector('.modal-close').onclick=function(){dialog.style.display='none';if(anchor)anchor.focus();};dialog.addEventListener('click',function(e){if(e.target===dialog)dialog.querySelector('.modal-close').click();});}
  dialog.querySelector('.server-config-name').textContent=card.dataset.name||'';dialog.querySelector('textarea').value='Загрузка…';dialog.querySelector('.server-config-error').textContent='';dialog.style.display='flex';dialog.querySelector('.modal-close').focus();
  try{var response=await fetch('/api/server_config?sid='+encodeURIComponent(card.dataset.sid),{cache:'no-store'});var data=await response.json();if(!response.ok)throw Error(data.error||'Не удалось загрузить конфиг');dialog.querySelector('textarea').value=JSON.stringify(data.config,null,2);}catch(error){dialog.querySelector('textarea').value='';dialog.querySelector('.server-config-error').textContent=error.message;}
 }
 document.addEventListener('DOMContentLoaded',function(){var root=document.getElementById('servers_container');if(!root)return;enhance(root);new MutationObserver(function(records){records.forEach(function(record){record.addedNodes.forEach(enhance);});}).observe(root,{subtree:true,childList:true});});
 document.addEventListener('click',function(e){var button=e.target.closest('.server-menu-trigger');if(button){e.preventDefault();e.stopPropagation();if(menu&&anchor===button){closeMenu();return;}closeMenu();anchor=button;menu=document.createElement('div');menu.id='server_actions_menu';menu.setAttribute('role','menu');var item=document.createElement('button');item.type='button';item.textContent='Просмотр конфига';item.setAttribute('role','menuitem');item.onclick=function(){viewConfig(button.closest('.server-card'));};menu.appendChild(item);document.body.appendChild(menu);button.setAttribute('aria-expanded','true');var rect=button.getBoundingClientRect();menu.style.left=Math.max(8,Math.min(rect.right-menu.offsetWidth,innerWidth-menu.offsetWidth-8))+'px';menu.style.top=(rect.bottom+menu.offsetHeight+6<innerHeight?rect.bottom+6:rect.top-menu.offsetHeight-6)+'px';item.focus();}else if(menu&&!menu.contains(e.target))closeMenu();});
 document.addEventListener('keydown',function(e){if(e.key==='Escape'){if(menu){closeMenu();if(anchor)anchor.focus();}else if(dialog&&dialog.style.display!=='none'){dialog.querySelector('.modal-close').click();}}});
 window.addEventListener('resize',closeMenu);document.addEventListener('scroll',closeMenu,true);
})();
/* Keep protocol summaries compact without repeating protocol as transport. */
document.addEventListener('DOMContentLoaded',function(){
 function updateSubtitles(){
  document.querySelectorAll('#servers_container .server-card .server-subtitle').forEach(function(line){
   var card=line.closest('.server-card'),protocol=(card.dataset.protocol||'').toUpperCase();
   var parts=line.textContent.split('/').map(function(value){return value.trim();}).filter(Boolean);
   var result=[];
   parts.forEach(function(value,index){
    if(index===0&&protocol)value=protocol;
    if(result.length&&value===result[result.length-1])return;
    if(index===1&&/^HYSTERIA2?$/.test(protocol)&&/^HYSTERIA2?$/.test(value))return;
    result.push(value);
   });
   var fragment=document.createDocumentFragment();
   result.forEach(function(value,index){
    if(index)fragment.appendChild(document.createTextNode(' / '));
    var badgeClass=value==='GRPC'?'badge-tr-grpc':/^HYSTERIA2?$/.test(value)?'badge-tr-hysteria':value==='BALANCER'?'badge-tr-balancer':'';
    if(badgeClass){var badge=document.createElement('span');badge.className='tr-badge '+badgeClass;badge.textContent=value;fragment.appendChild(badge);}
    else fragment.appendChild(document.createTextNode(value));
   });
   var holder=document.createElement('div');holder.appendChild(fragment);
   if(line.innerHTML!==holder.innerHTML)line.replaceChildren.apply(line,Array.from(holder.childNodes));
  });
 }
 updateSubtitles();
 var container=document.getElementById('servers_container');
 if(container)new MutationObserver(updateSubtitles).observe(container,{childList:true,subtree:true});
});

/* One OptiRout group-paste handler for button and keyboard submission. */
(function(){
 var routingApply=window.applyPasteGroup;
 function close(){
  var modal=document.getElementById('paste_group_modal');if(modal)modal.style.display='none';
  window.__mt_paste_ctx=null;window.__paste_group_ctx=null;
 }
 function open(button){
  window.__paste_group_ctx=null;window.__mt_paste_ctx={isMt:true,btn:button||null};
  var modal=document.getElementById('paste_group_modal');
  modal.querySelector('textarea').value='';
  selectPasteMode(modal.querySelector('.paste-mode-toggle button'),false);
  document.getElementById('paste_group_hint').textContent='Первая строка — название группы, далее правила. Ctrl+Enter — добавить.';
  modal.style.display='flex';setTimeout(function(){modal.querySelector('textarea').focus();},60);
 }
 function apply(){
  if(!window.__mt_paste_ctx||!window.__mt_paste_ctx.isMt){if(routingApply)return routingApply();return;}
  var hint=document.getElementById('paste_group_hint');
  var lines=document.getElementById('paste_group_textarea').value.split(/\r\n|\n|\r/).map(function(v){return v.trim();}).filter(function(v){return v&&v[0]!=='#';});
  if(lines.length<2){hint.textContent='Укажите название группы и хотя бы одно правило.';return;}
  var name=lines.shift(),rules=[],seen=new Set();
  lines.forEach(function(value){var parsed=window.mtParseImportedRule?window.mtParseImportedRule(value):{rule:value,type:'namespace'};var key=parsed.type+'\n'+parsed.rule;if(!seen.has(key)){seen.add(key);rules.push({id:_mtNewId(),name:'',type:parsed.type,rule:parsed.rule,enable:true});}});
  if(typeof _mtCollectFromUi==='function')_mtCollectFromUi(true);
  var state=window.MT_STATE,config=state&&state.config;
  if(!config){hint.textContent='Конфиг OptiRout не загружен.';return;}
  config.groups=config.groups||[];
  var group=config.groups.find(function(g){return g.name===name;});
  var replace=document.getElementById('paste_group_modal_replace').checked;
  if(group){
   if(replace)group.rules=rules;
   else {var existing=new Set((group.rules||[]).map(function(r){return r.type+'\n'+r.rule;}));group.rules=(group.rules||[]).concat(rules.filter(function(r){return !existing.has(r.type+'\n'+r.rule);}));}
   group.collapsed=false;
  }else{
   var iface=state.interfaces&&state.interfaces[0];
   config.groups.unshift({id:_mtNewId(),name:name,color:randomRoutingGroupColor(),interface:iface?(_mtIfaceName(iface)||'t2s0'):'t2s0',enable:true,rules:rules});
  }
  state.dirty=true;mtRenderGroups();close();
  _mtSetStatus('Группа «'+name+'» обновлена. Нажмите «Сохранить» для применения.','ok');
  if(window.__mtScanDups)setTimeout(window.__mtScanDups,150);
 }
 window.mtOpenPasteModal=open;window.mtPasteGroupFromClipboard=open;
 window.applyPasteGroupMt=apply;window.applyPasteGroupAny=apply;window.applyPasteGroup=apply;
 var previousClose=window.closePasteGroupModal;
 window.closePasteGroupModal=function(){if(previousClose)previousClose();window.__mt_paste_ctx=null;};
})();

document.addEventListener('DOMContentLoaded',function(){
 var status=document.querySelector('.header-right > .active-status-card');if(!status)return;
 var panel=document.createElement('div');panel.className='router-metrics-card';panel.innerHTML='<div class="router-model">Роутер</div><div class="router-metric"><span>ЦП</span><meter min="0" max="100" value="0"></meter><span class="router-cpu">—</span></div><div class="router-metric"><span>ОЗУ</span><meter min="0" max="100" value="0"></meter><span class="router-memory">—</span></div>';
 status.before(panel);var timer=null;
 function unavailable(){panel.querySelector('.router-cpu').textContent='Нет данных';panel.querySelector('.router-memory').textContent='Нет данных';panel.querySelectorAll('meter').forEach(function(m){m.value=0;});}
 async function update(){if(document.hidden)return;var controller=new AbortController(),timeout=setTimeout(function(){controller.abort();},2500);try{var response=await fetch('/api/router_metrics',{cache:'no-store',signal:controller.signal});if(!response.ok)throw Error();var data=await response.json();if(!data.ok)throw Error();var model=panel.querySelector('.router-model');model.textContent=data.model;var uptime=document.createElement('span');uptime.className='router-uptime';var seconds=Math.floor(data.uptime_seconds||0),days=Math.floor(seconds/86400),hours=Math.floor(seconds%86400/3600),minutes=Math.floor(seconds%3600/60);uptime.textContent=' ('+(days?days+' д. ':'')+hours+' ч. '+minutes+' мин.)';model.appendChild(uptime);var meters=panel.querySelectorAll('meter');meters[0].value=data.cpu_percent||0;meters[1].value=data.memory_percent;panel.querySelector('.router-cpu').textContent=data.cpu_percent==null?'…':Math.round(data.cpu_percent)+'%';var memory=panel.querySelector('.router-memory');memory.textContent=Math.round(data.memory_percent)+'%';var amount=document.createElement('span');amount.className='router-memory-amount';amount.textContent=' ('+data.memory_used_mb+' / '+data.memory_total_mb+' МБ)';memory.appendChild(amount);panel.querySelector('.router-memory').title=data.memory_used_mb+' / '+data.memory_total_mb+' МБ';}catch(e){unavailable();}finally{clearTimeout(timeout);if(!document.hidden)timer=setTimeout(update,3000);}}
 document.addEventListener('visibilitychange',function(){clearTimeout(timer);if(!document.hidden)update();});update();
});

/* Router actions are read-only until the explicit reboot confirmation. */
document.addEventListener('DOMContentLoaded',function(){
 var model=document.querySelector('.router-model');if(!model)return;
 model.tabIndex=0;model.setAttribute('role','button');model.setAttribute('aria-haspopup','menu');model.setAttribute('aria-label','Действия с роутером');
 var menu=document.createElement('div');menu.className='router-actions-popup';menu.hidden=true;menu.setAttribute('role','menu');
 var link=document.createElement('a');link.textContent='Открыть панель Keenetic';link.target='_blank';link.rel='noopener noreferrer';
 var host=document.querySelector('input[name="proxy_host"]');try{var address=new URL('http://'+(host&&host.value||location.hostname));if(address.protocol==='http:'&&!address.username&&!address.password)link.href=address.origin;}catch(e){link.href='http://'+location.hostname;}
 var reboot=document.createElement('button');reboot.type='button';reboot.textContent='Перезагрузить роутер';var externalIcon='<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><path d="M14 3h7v7M21 3 10 14M10 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-5"/></svg>';var restartIcon='<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><path d="M20 7v5h-5M20 12a8 8 0 1 0-2 6"/></svg>';link.insertAdjacentHTML('afterbegin',externalIcon);reboot.insertAdjacentHTML('afterbegin',restartIcon);menu.append(link,reboot);document.body.appendChild(menu);
 var timer;function close(){menu.hidden=true;model.setAttribute('aria-expanded','false');}function open(){clearTimeout(timer);menu.hidden=false;model.setAttribute('aria-expanded','true');var r=model.getBoundingClientRect();menu.style.left=Math.max(8,Math.min(r.left,innerWidth-menu.offsetWidth-8))+'px';menu.style.top=(r.bottom+6)+'px';}function later(){timer=setTimeout(close,180);}
 model.addEventListener('click',()=>menu.hidden?open():close());model.addEventListener('keydown',function(e){if(e.key==='Enter'||e.key===' '){e.preventDefault();open();link.focus();}});
 document.addEventListener('click',e=>{if(!menu.contains(e.target)&&!model.contains(e.target))close();});document.addEventListener('keydown',e=>{if(e.key==='Escape')close();});window.addEventListener('scroll',close,true);
 link.addEventListener('click',close);
 reboot.onclick=async function(){close();if(!confirm('Перезагрузить роутер? Интернет временно отключится.'))return;reboot.disabled=true;try{var response=await fetch('/api/router_reboot',{method:'POST',headers:{'X-OptiRay-Action':'reboot'}});var data=await response.json();if(!response.ok||!data.ok)throw Error(data.error||'Не удалось отправить команду');close();alert('Команда перезагрузки отправлена. Дождитесь восстановления связи с роутером.');}catch(e){alert(e.message);}finally{reboot.disabled=false;}};
});

/* Reopen settings from persisted values; never carry an abandoned edit forward. */
(function(){
 var originalOpen=window.openSettingsModal,originalClose=window.closeSettingsModal;
 var revision=0,baseline=[];
 function controls(){return Array.from(document.querySelectorAll('#settings_modal input,#settings_modal select,#settings_modal textarea')).filter(function(e){return e.type!=='file'&&e.name;});}
 function changed(){return baseline.some(function(item){var e=item.element;return e.isConnected&&((e.type==='checkbox'||e.type==='radio')?e.checked!==item.checked:e.value!==item.value);});}
 function updateSaveState(){var button=document.querySelector('#settings_modal button[name="action"][value="save_settings"]');if(!button)return;var dirty=changed();button.disabled=!dirty;button.setAttribute('aria-disabled',String(!dirty));button.classList.toggle('settings-has-changes',dirty);button.classList.toggle('settings-no-changes',!dirty);button.title=dirty?'Сохранить изменения':'Нет несохранённых изменений';}
 function capture(){baseline=controls().map(function(e){return {element:e,value:e.value,checked:e.checked};});updateSaveState();}
 function syncSelect(e){if(e.tagName!=='SELECT'||!e.id)return;var button=Array.from(document.querySelectorAll('#settings_modal .optiray-select-trigger')).find(function(b){return b.dataset.selectId===e.id;});if(button){var label=button.querySelector('span');if(label)label.textContent=e.selectedOptions[0]?e.selectedOptions[0].textContent:'';}}
 function restore(){baseline.forEach(function(item){var e=item.element;e.value=item.value;e.checked=item.checked;e.setCustomValidity('');syncSelect(e);});updateSaveState();}
 document.addEventListener('DOMContentLoaded',capture);
 ['input','change'].forEach(function(name){document.addEventListener(name,function(event){if(event.target.closest&&event.target.closest('#settings_modal'))updateSaveState();});});
 window.openSettingsModal=async function(){
  var current=++revision;restore();
  try{
   var response=await fetch('/',{cache:'no-store'});if(!response.ok)throw Error('HTTP '+response.status);
   var page=new DOMParser().parseFromString(await response.text(),'text/html');
   if(current!==revision)return;
   var saved=Array.from(page.querySelectorAll('#settings_modal input,#settings_modal select,#settings_modal textarea'));
   if(!saved.length)throw Error('Не удалось получить сохранённые настройки');
   controls().forEach(function(e){var source=saved.find(function(s){return s.name===e.name&&s.type===e.type&&(e.type!=='radio'||s.value===e.value);});if(!source)return;e.value=source.value;e.checked=source.checked;e.setCustomValidity('');syncSelect(e);});
   capture();originalOpen();
  }catch(e){if(current===revision)alert('Не удалось загрузить настройки: '+e.message);}
 };
 window.closeSettingsModal=function(){
  var modal=document.getElementById('settings_modal');
  if(modal&&modal.style.display==='flex'&&changed()&&!confirm('Есть несохранённые изменения. Закрыть настройки без сохранения?'))return false;
  revision++;restore();
  window.optiraySettingsInfoRevision=(window.optiraySettingsInfoRevision||0)+1;
  document.querySelectorAll('#settings_modal .core-versions-notice,#geo_data_status').forEach(function(el){el.textContent='';el.hidden=true;});
  originalClose();return true;
 };
})();

function syncGeoSource(){
 var standard=document.getElementById('geo_data_source').value==='standard';
 document.querySelectorAll('.geo-file-url').forEach(function(input){input.disabled=standard||!!window.optirayGeoBusy;});
}
function geoFieldError(input,message){input.setCustomValidity(message);if(message)input.reportValidity();return !message;}
function validateGeoURLs(name){
 var inputs=Array.from(document.querySelectorAll('.geo-file-url'));
 inputs.forEach(function(input){input.setCustomValidity('');});
 var input=document.querySelector('[data-geo-name="'+name+'"] .geo-file-url'),url;
 if(!input.value.trim())return geoFieldError(input,'Укажите прямую HTTPS-ссылку на '+name);
 try{url=new URL(input.value.trim());}catch(error){return geoFieldError(input,'Некорректная ссылка');}
 if(url.protocol!=='https:'||url.username||url.password||url.hash)return geoFieldError(input,'Нужна прямая HTTPS-ссылка на DAT-файл');
 var filename=decodeURIComponent(url.pathname.split('/').pop()).toLowerCase();
 if((filename==='geoip.dat'||filename==='geosite.dat')&&filename!==name)return geoFieldError(input,'Это ссылка на '+filename+'. Здесь нужен '+name);
 var other=inputs.find(function(field){return field!==input;});
 if(other&&other.value.trim()===input.value.trim())return geoFieldError(input,'Для GeoIP и GeoSite нужны разные ссылки');
 return true;
}
function renderGeoData(data,restoreFields){
 var source=document.getElementById('geo_data_source');
 if(restoreFields){source.value=data.source||'standard';source.dispatchEvent(new Event('change'));}
 (data.files||[]).forEach(function(file){
  var card=document.querySelector('[data-geo-name="'+file.name+'"]');if(!card)return;
  if(restoreFields){card.querySelector('.geo-file-url').value=file.url||'';card.querySelector('.geo-file-url').setCustomValidity('');}
  var size=file.size>=1048576?(file.size/1048576).toFixed(2)+' МБ':Math.ceil(file.size/1024)+' КБ';
  card.querySelector('.geo-data-file-info').textContent=file.exists?'Последнее обновление: '+new Date(file.updated_at).toLocaleString('ru-RU')+' · '+size:'Файл не установлен';
  card.querySelector('.geo-data-path').textContent=file.path;
 });syncGeoSource();
}
async function loadGeoData(){
 var infoRevision=window.optiraySettingsInfoRevision||0;
 if(window.optirayGeoBusy)return;
 var revision=window.optirayGeoRevision=(window.optirayGeoRevision||0)+1;
 document.getElementById('geo_data_status').textContent='';
 try{var response=await fetch('/api/geo_data',{cache:'no-store'});var data=await response.json();if(!response.ok||!data.ok)throw Error(data.error||'Не удалось прочитать информацию о базах');if(revision===window.optirayGeoRevision)renderGeoData(data,true);}
 catch(error){setSettingsInfo(document.getElementById('geo_data_status'),error.message,infoRevision);}
}
async function updateGeoData(button){
 var infoRevision=window.optiraySettingsInfoRevision||0;
 if(window.optirayGeoBusy)return;
 var source=document.getElementById('geo_data_source'),ip=document.getElementById('geo_geoip_url'),site=document.getElementById('geo_geosite_url'),status=document.getElementById('geo_data_status');
 if(source.value!=='standard'&&(!validateGeoURLs('geoip.dat')||!validateGeoURLs('geosite.dat')))return;
 window.optirayGeoBusy=true;window.optirayGeoRevision=(window.optirayGeoRevision||0)+1;
 button.disabled=source.disabled=true;document.querySelectorAll('.geo-file-url').forEach(function(field){field.disabled=true;});
 var trigger=document.querySelector('[data-select-id="geo_data_source"]');if(trigger)trigger.disabled=true;
 setSettingsInfo(status,'Скачивание и применение GeoIP и GeoSite…',infoRevision);
 try{var response=await fetch('/api/geo_data',{method:'POST',body:new URLSearchParams({source:source.value,geoip_url:ip.value.trim(),geosite_url:site.value.trim()})});var data=await response.json();if(!response.ok||!data.ok)throw Error(data.error||'Не удалось обновить базы');renderGeoData(data,false);setSettingsInfo(status,'GeoIP и GeoSite обновлены',infoRevision);if(window.refreshXrayGeoCategories)window.refreshXrayGeoCategories();}
 catch(error){setSettingsInfo(status,error.message,infoRevision);}
 finally{window.optirayGeoBusy=false;button.disabled=source.disabled=false;if(trigger)trigger.disabled=false;syncGeoSource();}
}
document.addEventListener('DOMContentLoaded',function(){
 if(window.optirayEnhanceSelect)window.optirayEnhanceSelect(document.getElementById('geo_data_source'));
 document.querySelectorAll('.geo-file-url').forEach(function(input){input.addEventListener('input',function(){document.querySelectorAll('.geo-file-url').forEach(function(field){field.setCustomValidity('');});});});
});

async function refreshCoreVersions(kind,button){
 if((kind==='xray'&&xrayVersionsLoading)||(kind==='hysteria'&&hysteriaVersionsLoading))return;
 button.disabled=true;button.classList.add('is-loading');
 try{if(kind==='xray')await loadXrayVersions(true);else await checkHysteria(true);}
 finally{button.disabled=false;button.classList.remove('is-loading');}
}

function setSettingsInfo(element,message,revision){
 if(!element)return;
 var modal=document.getElementById('settings_modal');
 if(!modal||modal.style.display!=='flex'||revision!==(window.optiraySettingsInfoRevision||0))return;
 element.textContent=message;element.hidden=!message;
}

/* Xray geo categories and separate address/IP editors share the existing rule format. */
(function(){
 'use strict';
 var cats=['block','direct','proxy'],catalog=null,catalogRevision=0,picker=null,pickerState=null;
 function canonical(cat){return document.querySelector('#routing_modal [name="routing_'+cat+'_rules"]');}
 function isIPRule(value){
  var s=value.trim();if(/^(?:geoip|ip|ext-ip):/i.test(s))return true;
  var v4='(?:\\d{1,3}\\.){3}\\d{1,3}';
  if(new RegExp('^'+v4+'(?:/\\d{1,2}|\\s*-\\s*'+v4+')?$').test(s))return true;
  return s.indexOf(':')>=0&&/^[0-9a-f:.]+(?:\/\d{1,3})?$/i.test(s);
 }
 function splitRules(value){var result={addresses:[],ips:[]};String(value||'').split(/\r?\n/).forEach(function(line){(isIPRule(line)?result.ips:result.addresses).push(line);});return result;}
 function splitBox(cat){return document.querySelector('.xr-split-fields[data-cat="'+cat+'"]');}
 window.syncXraySplitFields=function(){
  cats.forEach(function(cat){var original=canonical(cat),box=splitBox(cat);if(!original||!box||box.dataset.serialized===original.value)return;
   var parts=splitRules(original.value);box.querySelector('[data-rule-side="addresses"]').value=parts.addresses.join('\n');box.querySelector('[data-rule-side="ips"]').value=parts.ips.join('\n');box.dataset.serialized=original.value;
  });
 };
 function combine(cat){
  var box=splitBox(cat),original=canonical(cat);if(!box||!original)return;
  var left=box.querySelector('[data-rule-side="addresses"]').value,right=box.querySelector('[data-rule-side="ips"]').value;
  original.value=[left,right].filter(function(v){return v.trim();}).join('\n');box.dataset.serialized=original.value;
  original.dispatchEvent(new Event('input',{bubbles:true}));
 }
 function geoReference(value){var match=String(value||'').trim().match(/^(geosite|geoip):(!?)([^\s@]+)((?:@[^\s@]+)*)$/i);return match?{kind:match[1].toLowerCase(),code:match[3].toLowerCase(),attrs:match[4].split('@').filter(Boolean).map(function(v){return v.toLowerCase();})}:null;}
 function missingReference(ref){
  if(!catalog||!ref)return '';
  var file=catalog[ref.kind];if(!file||!file.exists)return 'Не установлен '+ref.kind+'.dat';
  if(file.error)return file.error;
  var category=(file.categories||[]).find(function(item){return item.code===ref.code;});
  if(!category)return 'В '+ref.kind+'.dat нет категории '+ref.code;
  if(ref.attrs.some(function(attr){return (category.attributes||[]).indexOf(attr)<0;}))return 'В категории '+ref.code+' отсутствует выбранный атрибут';
  return '';
 }
 window.validateXrayGeoReferences=function(){
  document.querySelectorAll('#routing_modal .rt-rule-input').forEach(function(input){
   var row=input.closest('.rt-rule'),ref=geoReference(input.value),badge=row.querySelector('.xr-geo-badge'),warning=missingReference(ref);
   if(ref){if(!badge){badge=document.createElement('span');badge.className='xr-geo-badge';input.after(badge);}var text=ref.kind==='geoip'?'GeoIP':'GeoSite';if(badge.textContent!==text)badge.textContent=text;badge.title=warning||ref.kind+':'+ref.code;}
   else if(badge)badge.remove();
   row.classList.toggle('xr-geo-missing',!!warning);if(warning)input.title=warning;else if(input.dataset.geoWarning)input.removeAttribute('title');input.dataset.geoWarning=warning;
  });
  document.querySelectorAll('#routing_modal .rt-group-card').forEach(function(card){
   var missing=card.querySelector('.xr-geo-missing'),heading=card.querySelector('.rt-group-head');card.classList.toggle('xr-geo-group-missing',!!missing);
   if(heading){if(missing){heading.title='В группе есть отсутствующие категории геобазы';heading.dataset.geoWarning='true';}else if(heading.dataset.geoWarning){heading.removeAttribute('title');delete heading.dataset.geoWarning;}}
  });
  var duplicates=window.xrayDuplicateEntries?window.xrayDuplicateEntries():[];
  document.querySelectorAll('.xr-split-fields textarea').forEach(function(input){
   var warnings=[];input.value.split(/\r?\n/).forEach(function(line){var warning=missingReference(geoReference(line));if(warning&&warnings.indexOf(warning)<0)warnings.push(warning);});
   var duplicateNames=duplicates.filter(function(entry){return entry.input===input;}).map(function(entry){return entry.value;});if(duplicateNames.length)warnings.unshift('Повторяющиеся правила: '+Array.from(new Set(duplicateNames)).slice(0,3).join(', '));
   var message=input.parentNode.querySelector('.xr-geo-list-warning');message.textContent=warnings.slice(0,3).join('; ')+(warnings.length>3?'…':'');message.hidden=!warnings.length;input.classList.toggle('xr-geo-missing',!!warnings.length);
  });
  document.querySelectorAll('#routing_modal .rt-rule-input').forEach(function(input){var repeated=duplicates.some(function(entry){return entry.input===input;});input.classList.toggle('xr-rule-duplicate',repeated);if(repeated)input.title='Повторяющееся правило';else if(input.dataset.duplicateWarning&&!input.dataset.geoWarning)input.removeAttribute('title');input.dataset.duplicateWarning=repeated?'true':'';});
 };
 window.refreshXrayGeoCategories=async function(){
  var revision=++catalogRevision;
  try{var response=await fetch('/api/geo_categories',{cache:'no-store'}),data=await response.json();if(!response.ok||!data.ok)throw Error('Не удалось прочитать геобазы');if(revision!==catalogRevision)return;catalog=data;window.validateXrayGeoReferences();return true;}
  catch(error){if(pickerState){picker.querySelector('.geo-picker-message').textContent=error.message;}return false;}
 };
 function existingRules(cat){
  var values=new Set(),data=window.__rt_get_data?window.__rt_get_data():{};
  cats.forEach(function(category){
   (canonical(category).value||'').split(/\r?\n/).forEach(function(v){var key=xrayRuleKey(v);if(key)values.add(key);});
   (data[category]||[]).forEach(function(group){(group.rules||[]).forEach(function(rule){var key=xrayRuleKey(typeof rule==='string'?rule:rule.value);if(key)values.add(key);});});
  });return values;
 }
 function closePicker(){if(!pickerState)return;var target=pickerState.returnFocus;pickerState=null;picker.hidden=true;if(target&&target.isConnected)target.focus();}
 function drawPicker(){
  if(!pickerState)return;
  var state=pickerState,kind=state.kind,file=catalog&&catalog[kind],list=picker.querySelector('.geo-picker-list'),message=picker.querySelector('.geo-picker-message'),query=picker.querySelector('input[type="search"]').value.trim().toLowerCase();list.replaceChildren();message.textContent='';
  picker.querySelectorAll('[data-geo-kind]').forEach(function(button){button.setAttribute('aria-pressed',String(button.dataset.geoKind===kind));});
  if(!file||!file.exists)message.textContent='Файл '+kind+'.dat не установлен. Добавьте его в Настройки → Ядра.';
  else if(file.error)message.textContent=file.error;
  else {
   var entries=[];(file.categories||[]).forEach(function(item){entries.push({code:item.code,count:item.count});(item.attributes||[]).forEach(function(attr){entries.push({code:item.code+'@'+attr,count:null});});});
   var visible=entries.filter(function(item){return item.code.indexOf(query)>=0;});
   if(!visible.length)message.textContent='Категории не найдены';
   var fragment=document.createDocumentFragment();visible.forEach(function(item){
    var rule=kind+':'+item.code,label=document.createElement('label'),check=document.createElement('input'),text=document.createElement('span'),detail=document.createElement('small');label.className='geo-picker-item';check.type='checkbox';check.checked=state.selected.has(rule)||state.existing.has(xrayRuleKey(rule));check.disabled=state.existing.has(xrayRuleKey(rule));text.textContent=item.code;detail.textContent=check.disabled?'Уже добавлено':item.count===null?'Атрибут':item.count+' записей';
    check.addEventListener('change',function(){if(check.checked)state.selected.add(rule);else state.selected.delete(rule);updatePickerButton();});var mark=document.createElement('span');mark.className='geo-picker-check';mark.setAttribute('aria-hidden','true');mark.innerHTML='<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m3 8 3 3 7-7"/></svg>';label.append(check,mark,text,detail);fragment.append(label);
   });list.append(fragment);
  }
  updatePickerButton();
 }
 function updatePickerButton(){var button=picker.querySelector('.geo-picker-add');button.disabled=!pickerState||!pickerState.selected.size;button.textContent=pickerState&&pickerState.selected.size?'Добавить ('+pickerState.selected.size+')':'Добавить выбранные';}
 function ensurePicker(){
  if(picker)return;
  picker=document.createElement('div');picker.className='geo-picker-modal';picker.hidden=true;picker.setAttribute('role','dialog');picker.setAttribute('aria-modal','true');picker.setAttribute('aria-labelledby','geo_picker_title');
  picker.innerHTML='<div class="geo-picker-card"><div class="geo-picker-header modal-header"><span class="modal-title" id="geo_picker_title">Из геобазы</span><button type="button" class="geo-picker-close modal-close" aria-label="Закрыть"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg></button></div><div class="geo-picker-content"><div class="geo-picker-tabs" role="group" aria-label="Тип базы"><button type="button" data-geo-kind="geosite">GeoSite</button><button type="button" data-geo-kind="geoip">GeoIP</button></div><input type="search" placeholder="Поиск категории" aria-label="Поиск категории"><div class="geo-picker-message" role="status"></div><div class="geo-picker-list"></div></div><div class="geo-picker-footer"><button type="button" class="geo-picker-settings">Настройки баз</button><button type="button" class="geo-picker-cancel">Отмена</button><button type="button" class="geo-picker-add core-btn-install">Добавить выбранные</button></div></div>';
  document.body.append(picker);
  picker.querySelector('.geo-picker-close').onclick=closePicker;picker.querySelector('.geo-picker-cancel').onclick=closePicker;picker.addEventListener('click',function(event){if(event.target===picker)closePicker();});
  picker.querySelectorAll('[data-geo-kind]').forEach(function(button){button.onclick=function(){pickerState.kind=button.dataset.geoKind;drawPicker();};});picker.querySelector('input[type="search"]').oninput=drawPicker;
  picker.querySelector('.geo-picker-add').onclick=function(){
   if(!pickerState)return;var cat=pickerState.cat,existing=existingRules(cat),selected=Array.from(pickerState.selected).filter(function(rule){return !existing.has(xrayRuleKey(rule));});
   if(selected.length){var original=canonical(cat);original.value=selected.join('\n')+(original.value?'\n'+original.value:'');var box=splitBox(cat);if(box)delete box.dataset.serialized;window.syncXraySplitFields();original.dispatchEvent(new Event('input',{bubbles:true}));}
   closePicker();window.validateXrayGeoReferences();
  };
  picker.querySelector('.geo-picker-settings').onclick=async function(){
   closePicker();var routing=document.getElementById('mt_modal');routing.style.display='none';window.optirayGeoReturnRouting=true;
   await window.openSettingsModal();var settings=document.getElementById('settings_modal');
   if(settings.style.display==='flex')window.selectSettingsTab('cores');else{routing.style.display='flex';window.optirayGeoReturnRouting=false;}
  };
  window.addEventListener('keydown',function(event){
   if(!pickerState)return;
   if(event.key==='Escape'){event.preventDefault();event.stopImmediatePropagation();closePicker();}
   if(event.key==='Tab'){var fields=Array.from(picker.querySelectorAll('button,input')).filter(function(el){return !el.disabled&&el.getClientRects().length;});var first=fields[0],last=fields[fields.length-1];if(event.shiftKey&&document.activeElement===first){event.preventDefault();last.focus();}else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first.focus();}event.stopImmediatePropagation();}
  },true);
 }
 async function openPicker(cat,button){
  ensurePicker();pickerState={cat:cat,kind:'geosite',selected:new Set(),existing:existingRules(cat),returnFocus:button};picker.hidden=false;picker.querySelector('#geo_picker_title').textContent='Из геобазы — '+({block:'Блокировать',direct:'Напрямую',proxy:'Через прокси'}[cat]);picker.querySelector('input[type="search"]').value='';picker.querySelector('input[type="search"]').focus();picker.querySelector('.geo-picker-message').textContent='Чтение категорий…';
  var state=pickerState;picker.querySelector('.geo-picker-list').replaceChildren();var loaded=await window.refreshXrayGeoCategories();if(pickerState===state&&loaded)drawPicker();
 }
 document.addEventListener('DOMContentLoaded',function(){
  cats.forEach(function(cat){var original=canonical(cat);if(!original)return;original.classList.add('xr-combined-rules');
   var box=document.createElement('div');box.className='xr-split-fields';box.dataset.cat=cat;
   ['addresses','ips'].forEach(function(side){var field=document.createElement('div'),label=document.createElement('label'),input=document.createElement('textarea'),warning=document.createElement('div');field.className='xr-split-field';label.textContent=side==='ips'?'IP':'Адреса';input.id='xr_'+cat+'_'+side;label.htmlFor=input.id;input.dataset.ruleSide=side;input.className='setting-input';input.rows=4;input.spellcheck=false;input.placeholder=side==='ips'?'IP, подсети, geoip:…':'Домены, geosite:…';warning.className='xr-geo-list-warning';warning.hidden=true;input.addEventListener('input',function(){combine(cat);window.validateXrayGeoReferences();});field.append(label,input,warning);box.append(field);});original.after(box);
   var row=document.getElementById('rt_groups_'+cat).parentNode.querySelector('.rt-add-group-row'),button=document.createElement('button');button.type='button';button.className='xr-add-geo';button.textContent='Из геобазы';button.addEventListener('click',function(){openPicker(cat,button);});var tools=document.createElement('div');tools.className='xr-rule-tools';row.before(tools);tools.append(row,button);
  });
  window.syncXraySplitFields();
  var editor=document.getElementById('routing_modal');['input','change'].forEach(function(name){editor.addEventListener(name,function(){window.validateXrayGeoReferences();});});
  var close=window.closeSettingsModal;window.closeSettingsModal=function(){var result=close.apply(this,arguments);if(result===false)return false;if(window.optirayGeoReturnRouting){window.optirayGeoReturnRouting=false;document.getElementById('mt_modal').style.display='flex';if(window.optiraySettingsReturnFocus)window.optiraySettingsReturnFocus.focus();}};
 });
})();

function xrayRuleKey(value){
 var text=String(value||'').trim();if(!text||text[0]==='#')return '';
 var lower=text.toLowerCase();if(lower.indexOf('regexp:')===0)return 'regexp:'+text.slice(7);
 if(lower.indexOf('ip:')===0)lower=lower.slice(3).trim();
 if(/^[0-9a-f:.]+$/i.test(lower)&&lower.indexOf(':')>=0){try{return 'ip:'+new URL('http://['+lower+']/').hostname.slice(1,-1);}catch(error){}}
 if(/^(?:\d{1,3}\.){3}\d{1,3}$/.test(lower))return 'ip:'+lower;
 if(lower.indexOf('domain:')===0)lower=lower.slice(7).trim();
 if(lower.indexOf(':')<0)lower=lower.replace(/^\*\./,'').replace(/\.+$/,'');return lower;
}
window.xrayGeoRulesFirst=function(value){var geo=[],other=[];String(value||'').split(/\r?\n/).forEach(function(line){(/^(?:geosite|geoip):/i.test(line.trim())?geo:other).push(line);});return geo.concat(other).join('\n');};
window.xrayDuplicateEntries=function(){
 var entries=[],map=new Map();
 function add(value,cat,input){var key=xrayRuleKey(value);if(!key)return;var entry={value:value.trim(),cat:cat,input:input};if(!map.has(key))map.set(key,[]);map.get(key).push(entry);}
 ['block','direct','proxy'].forEach(function(cat){
  document.querySelectorAll('.xr-split-fields[data-cat="'+cat+'"] textarea').forEach(function(input){input.value.split(/\r?\n/).forEach(function(line){add(line,cat,input);});});
  document.querySelectorAll('#rt_groups_'+cat+' .rt-rule-input').forEach(function(input){var row=input.closest('.rt-rule'),group=input.closest('.rt-group-card');if(row.classList.contains('rt-rule-off')||group.classList.contains('rt-group-off'))return;add(input.value,cat,input);});
 });
 map.forEach(function(values){if(values.length>1)entries.push.apply(entries,values);});return entries;
};

// Compare persisted routing values; UI expansion, generated IDs and empty rows do not count.
function routingConfigSignature(kind,value){
 var data=JSON.parse(JSON.stringify(value||{}));
 function cleanGroup(group){
  delete group.id;delete group.collapsed;delete group._collapsed;
  if(kind==='optirout')group.enable=group.enable!==false;else group.enabled=group.enabled!==false;
  group.rules=(group.rules||[]).map(function(rule){
   if(typeof rule==='string')return {value:rule.trim(),enabled:true};
   delete rule.id;delete rule.collapsed;
   if(kind==='optirout'){rule.rule=String(rule.rule||'').trim();rule.enable=rule.enable!==false;}
   else{rule.value=String(rule.value||'').trim();rule.enabled=rule.enabled!==false;}
   return rule;
  }).filter(function(rule){return !!(kind==='optirout'?rule.rule:rule.value);});return group;
 }
 // Engine power is applied immediately, outside the Save transaction.
 delete data.enabled;
 if(kind==='optirout')data.groups=(data.groups||[]).map(cleanGroup);
 else{
  ['block','direct','proxy'].forEach(function(cat){
   if(data.groups_ui&&data.groups_ui[cat])data.groups_ui[cat]=data.groups_ui[cat].map(cleanGroup);
   data[cat+'_rules']=String(data[cat+'_rules']||'').split(/\r?\n/).map(function(line){return line.trim();}).filter(Boolean).join('\n');
  });
 }
 function stable(item){if(Array.isArray(item))return item.map(stable);if(item&&typeof item==='object'){var result={};Object.keys(item).sort().forEach(function(key){result[key]=stable(item[key]);});return result;}return item;}
 return JSON.stringify(stable(data));
}

(function(){
 var loading=false,timer=null;
 function badge(parent,visible,title,corner){
  if(!parent)return;var item=parent.querySelector(':scope > .core-update-badge');
  if(!item){item=document.createElement('span');item.className='core-update-badge'+(corner?' core-update-corner':'');item.setAttribute('role','img');item.innerHTML='<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M8 12V4m-4 4 4-4 4 4"/></svg>';parent.append(item);}
  item.hidden=!visible;item.title=title;item.setAttribute('aria-label',title);
 }
 function render(data){
  var any=false,titles=[];
  ['xray','hysteria'].forEach(function(kind){var core=data[kind]||{},visible=!!core.update_available;
   var title='Доступна новая версия '+(kind==='xray'?'Xray':'Hysteria')+': v'+(core.latest||'');
   var heading=document.querySelector('.settings-core-heading[data-core-kind="'+kind+'"]');
   if(heading){var oldBadge=heading.querySelector(':scope > .core-update-badge');if(oldBadge)oldBadge.remove();var notice=heading.querySelector('.core-update-label');if(!notice){notice=document.createElement('span');notice.className='core-update-label';notice.textContent='Доступно обновление';heading.append(notice);}notice.hidden=!visible;notice.title=title;}
   if(visible){any=true;titles.push(title);}
  });
  var title=titles.join('; ');badge(document.getElementById('settings_open_button'),any,title,true);badge(document.querySelector('#settings_modal [data-settings-tab="cores"]'),any,title,false);
 }
 window.refreshCoreUpdateBadges=async function(){
  if(loading||document.hidden)return;loading=true;clearTimeout(timer);
  try{var response=await fetch('/api/core_update_status',{cache:'no-store'});if(response.ok){var data=await response.json();if(data.ok)render(data);}}
  catch(error){}finally{loading=false;if(!document.hidden)timer=setTimeout(window.refreshCoreUpdateBadges,60000);}
 };
 document.addEventListener('DOMContentLoaded',window.refreshCoreUpdateBadges);
 document.addEventListener('visibilitychange',function(){clearTimeout(timer);if(!document.hidden)window.refreshCoreUpdateBadges();});
})();
