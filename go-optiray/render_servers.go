package main

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
)

func esc(v any) string { return html.EscapeString(str(v)) }
func colorOf(sub Object) string {
	v := str(sub["color"])
	if !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(v) {
		return "#1971c2"
	}
	return v
}
func ruServers(n int) string {
	word := "серверов"
	if n%100 < 11 || n%100 > 14 {
		if n%10 == 1 {
			word = "сервер"
		} else if n%10 >= 2 && n%10 <= 4 {
			word = "сервера"
		}
	}
	return fmt.Sprintf("%d %s", n, word)
}

const heartIcon = `<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/></svg>`
const dragIcon = `<svg viewBox="0 0 24 24" width="22" height="24" fill="currentColor" aria-hidden="true"><circle cx="9" cy="5" r="1.6"/><circle cx="15" cy="5" r="1.6"/><circle cx="9" cy="12" r="1.6"/><circle cx="15" cy="12" r="1.6"/><circle cx="9" cy="19" r="1.6"/><circle cx="15" cy="19" r="1.6"/></svg>`

const paletteIcon = `<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3a9 9 0 0 0 0 18c1.1 0 2-.9 2-2 0-.5-.2-.95-.5-1.3-.3-.35-.5-.8-.5-1.2 0-1 .8-1.8 1.8-1.8H16a5 5 0 0 0 5-5c0-3.9-4-6.7-9-6.7z"/><circle cx="7.5" cy="10.5" r="1.1"/><circle cx="11" cy="7.5" r="1.1"/><circle cx="15" cy="8" r="1.1"/></svg>`
const deleteIcon = `<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 6h18M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M5 6l1 14a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1l1-14M10 10v7m4-7v7"/></svg>`
const tcpIcon = `<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M7 3 3 7l4 4M3 7h17M17 13l4 4-4 4M21 17H4"/></svg>`
const httpIcon = `<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18"/></svg>`
const stopIcon = `<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><rect x="4" y="4" width="16" height="16" rx="2"/></svg>`
const openIcon = `<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M18 13v6H5V6h6M15 3h6v6M10 14 21 3"/></svg>`

func serverDisplayName(name string) string {
	r := []rune(name)
	if len(r) >= 2 && r[0] >= 0x1f1e6 && r[0] <= 0x1f1ff && r[1] >= 0x1f1e6 && r[1] <= 0x1f1ff {
		return strings.TrimSpace(strings.TrimLeft(string(r[2:]), "\ufe0f\u200d"))
	}
	return name
}
func serverSubtitle(s Object) string {
	transport := strings.ToUpper(str(s["transport"]))
	class := ""
	if strings.EqualFold(transport, "grpc") {
		class = "badge-tr-grpc"
	} else if str(s["protocol"]) == "hysteria" || str(s["protocol"]) == "hysteria2" {
		class = "badge-tr-hysteria"
	}
	tr := esc(transport)
	if class != "" {
		tr = `<span class="tr-badge ` + class + `">` + tr + `</span>`
	}
	return esc(strings.ToUpper(str(s["protocol"]))) + " / " + tr + " / " + esc(strings.ToUpper(str(s["security"]))) + " / " + esc(strings.ToUpper(str(s["format"])))
}

func renderSubscriptionInputs(d Object) (string, string) {
	var rows, chips strings.Builder
	for _, v := range list(d["subscriptions"]) {
		sub := obj(v)
		id := integer(sub["id"])
		color := colorOf(sub)
		checked := ""
		if boolean(sub["use_hwid"]) {
			checked = "checked"
		}
		name := esc(sub["name"])
		fmt.Fprintf(&rows, `<div id="sub-row-%d" class="sub-row" data-sub-id="%d" style="background:%s10;border:2px solid %s40;border-left:16px solid %s80;"><div class="sub-row-inputs"><div style="display:flex;align-items:center;gap:12px"><div class="drag-handle" title="Перетащить">%s</div><div class="sub-title-badge">%s</div></div><input type="text" name="sub_url_%d" value="%s" placeholder="https://..." class="sub-url-input"></div><div class="sub-row-controls"><div class="color-picker-group"><input type="color" name="sub_color_%d" value="%s" onchange="changeSubColor(%d,this.value)" class="color-btn" title="Цвет"><button type="button" onclick="randomSubColor(Number(this.closest('.sub-row').dataset.subId))" class="dice-btn" title="Случайный цвет">`+paletteIcon+`</button></div><label class="hwid-label"><input type="checkbox" name="sub_hwid_%d" value="1" %s> HWID</label><button type="submit" name="action" value="del_%d" class="del-btn" aria-label="Удалить подписку" onclick="return confirm('Удалить подписку?')">`+deleteIcon+`</button></div></div>`, id, id, color, color, color, dragIcon, name, id, esc(sub["url"]), id, color, id, id, checked, id)
		fmt.Fprintf(&chips, `<span class="subs-name-chip text-style-7" style="background:%s10;border-color:%s40"><span class="subs-name-dot" style="background:%s"></span>%s</span>`, color, color, color, name)
	}
	return rows.String(), chips.String()
}
func renderServerCards(d Object) string {
	var out strings.Builder
	favorite := map[string]bool{}
	for _, v := range list(d["favorites"]) {
		favorite[str(v)] = true
	}
	selected := str(d["selected"])
	if !optirayEnabled(d) {
		selected = ""
	}
	counter := 0
	for _, v := range list(d["subscriptions"]) {
		sub := obj(v)
		servers := list(sub["servers"])
		if len(servers) == 0 {
			continue
		}
		id := integer(sub["id"])
		color := colorOf(sub)
		fmt.Fprintf(&out, `<div id="sub-section-%d" class="sub-section" data-subname="%s" data-subcolor="%s" style="background:%s10;border:2px solid %s40;border-left:8px solid %s80"><div class="sub-header"><div class="sub-title"><button type="button" class="btn-toggle-collapse" onclick="toggleSubCollapse(%d)" id="collapse_btn_%d">▼ Свернуть</button><span>%s</span><span class="sub-count">(%s)</span>`, id, esc(sub["name"]), color, color, color, color, id, id, esc(sub["name"]), ruServers(len(servers)))
		if boolean(sub["stale"]) {
			out.WriteString(`<span class="stale-badge" title="Последнее обновление не удалось">⚠ устарело</span>`)
		}
		if boolean(sub["use_hwid"]) {
			out.WriteString(`<span class="hwid-badge">HWID ON</span>`)
		}
		if boolean(d["show_test_button"]) && validHTTPURL(str(d["test_button_url"])) {
			fmt.Fprintf(&out, `<a href="%s" target="_blank" rel="noopener noreferrer" class="btn-test-site">`+openIcon+`Тест</a>`, esc(d["test_button_url"]))
		}
		fmt.Fprintf(&out, `</div><div class="sub-actions"><button type="button" class="btn-auto-select text-style-7" onclick="autoSelectSub(%d,this)">Автовыбор</button><div class="ping-block"><div class="ping-block-buttons"><span class="ping-progress" id="ping_progress_%d" role="status"></span><button type="button" onclick="pingGroupFavs(%d,'real')" class="btn-sub-fav-ping">%sFAV</button><button type="button" onclick="pingGroup(%d,'tcp')" class="btn-sub-tcppool">`+tcpIcon+`TCP</button><button type="button" onclick="pingGroup(%d,'real')" class="btn-sub-ping">`+httpIcon+`HTTP</button><button type="button" onclick="stopPing()" class="btn-sub-stop">`+stopIcon+`Стоп</button></div></div></div></div><div class="no-favs-msg" id="no_favs_msg_%d" style="display:none">Избранных или подключенных серверов нет.</div><div class="server-grid" id="grid_sub_%d">`, id, id, id, heartIcon, id, id, id, id)
		indices := make([]int, len(servers))
		for i := range indices {
			indices[i] = i
		}
		rank := func(i int) int {
			sid := str(obj(servers[i])["sid"])
			if sid == selected && sid != "" {
				return 0
			}
			if favorite[sid] {
				return 1
			}
			return 2
		}
		sort.SliceStable(indices, func(i, j int) bool { return rank(indices[i]) < rank(indices[j]) })
		for _, index := range indices {
			srv := obj(servers[index])
			counter++
			cid := fmt.Sprintf("srv_%d", counter)
			sid := str(srv["sid"])
			active := sid != "" && sid == selected
			isFav := favorite[sid]
			activeClass, favClass, activeVal, favVal := "", "", "0", "0"
			if active {
				activeClass = "active-card"
				activeVal = "1"
			}
			icon := heartIcon
			if isFav {
				favClass = "active"
				favVal = "1"
				icon = strings.Replace(icon, `fill="none"`, `fill="currentColor"`, 1)
			}
			fmt.Fprintf(&out, `<div class="server-card sub-group-%d %s" data-name="%s" data-sid="%s" data-source-order="%d" data-fav="%s" data-active="%s" data-transport="%s" data-protocol="%s" id="%s"><div style="margin-bottom:8px"><div style="display:flex;justify-content:space-between;align-items:flex-start;gap:8px;margin-bottom:4px"><div style="display:flex;align-items:center;gap:8px;flex:1;min-width:0"><button type="button" class="fav-btn %s" onclick="toggleFav(this,'%s')">%s</button>%s<div class="server-name" title="%s">%s</div></div><span class="ping-badge" onclick="pingSingle('%s','real')">-- ms</span></div><div class="server-subtitle">%s</div></div><div class="action-slot">`, id, activeClass, esc(srv["name"]), esc(sid), index, favVal, activeVal, esc(strings.ToLower(str(srv["transport"]))), esc(strings.ToLower(str(srv["protocol"]))), cid, favClass, cid, icon, serverFlag(str(srv["name"])), esc(serverDisplayName(str(srv["name"]))), esc(serverDisplayName(str(srv["name"]))), cid, serverSubtitle(srv))
			if active {
				out.WriteString(`<div class="badge-connected text-style-7">Подключено</div>`)
			} else if !supportedServer(srv) {
				fmt.Fprintf(&out, `<div class="badge-unsupported">⚠ %s</div>`, esc(first(str(srv["unsupported_reason"]), "Требуется отдельное ядро")))
			} else {
				out.WriteString(`<button type="button" onclick="connectServer(this)" class="btn-connect text-style-7">Подключить</button>`)
			}
			out.WriteString(`</div></div>`)
		}
		out.WriteString(`</div></div>`)
	}
	if counter == 0 {
		return `<div style="text-align:center;margin:30px 0">Список серверов пуст. Вставьте ссылку и нажмите «Обновить подписки».</div>`
	}
	return out.String()
}
func validHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
func serverFlag(name string) string {
	runes := []rune(name)
	for i := 0; i+1 < len(runes); i++ {
		if runes[i] >= 0x1f1e6 && runes[i] <= 0x1f1ff && runes[i+1] >= 0x1f1e6 && runes[i+1] <= 0x1f1ff {
			code := string([]rune{'a' + runes[i] - 0x1f1e6, 'a' + runes[i+1] - 0x1f1e6})
			return fmt.Sprintf(`<img class="srv-flag" src="/api/flag/%s" width="22" height="16" alt="%s" loading="eager">`, code, strings.ToUpper(code))
		}
	}
	data, _ := assets.ReadFile("assets/countries.json")
	countries := map[string]string{}
	_ = json.Unmarshal(data, &countries)
	names := make([]string, 0, len(countries))
	for name := range countries {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) != len(names[j]) {
			return len(names[i]) > len(names[j])
		}
		return names[i] < names[j]
	})
	lower := strings.ToLower(name)
	for _, country := range names {
		if strings.Contains(lower, country) {
			code := strings.ToLower(countries[country])
			return fmt.Sprintf(`<img class="srv-flag" src="/api/flag/%s" width="22" height="16" alt="%s" loading="eager">`, code, strings.ToUpper(code))
		}
	}
	return ""
}
